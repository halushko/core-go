package sqlite

import (
	"strings"
	"testing"
	"time"
)

type testRecord struct {
	ID        int64     `sqlite:"id,pk,autoincrement"`
	DeviceID  string    `sqlite:"device_id,notnull,index=idx_events_device,unique_group=u_device_time"`
	CreatedAt time.Time `sqlite:"created_at,notnull,unique_group=u_device_time"`
	Online    bool      `sqlite:"online,default=0"`
	Voltage   float64   `sqlite:"voltage,check=voltage >= 0"`
	Payload   []byte    `sqlite:"payload"`
	ParentID  *int64    `sqlite:"parent_id,references=parents(id),ondelete=SET NULL"`
	Ignored   string    `sqlite:"-"`
}

func TestTableFromStruct(t *testing.T) {
	table, err := TableFromStruct("events", &testRecord{})
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Columns) != 7 || len(table.Indexes) != 1 || len(table.ForeignKeys) != 1 {
		t.Fatalf("unexpected table: %#v", table)
	}
	if table.Columns[0].Autoincrement == nil || !*table.Columns[0].Autoincrement {
		t.Fatalf("missing autoincrement")
	}
	sql := buildCreateTableSQL(table, true)
	for _, fragment := range []string{
		`"id" INTEGER PRIMARY KEY AUTOINCREMENT`,
		`"created_at" DATETIME NOT NULL`,
		`"online" INTEGER DEFAULT 0`,
		`CHECK (voltage >= 0)`,
		`FOREIGN KEY ("parent_id") REFERENCES "parents" ("id") ON DELETE SET NULL`,
		`UNIQUE ("device_id", "created_at")`,
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("SQL missing %q: %s", fragment, sql)
		}
	}
	if strings.Contains(sql, "ignored") {
		t.Errorf("ignored field present: %s", sql)
	}
}

func TestCreateTableFromStruct(t *testing.T) {
	t.Setenv("DB_PATH", t.TempDir())
	c, err := Open("struct_schema")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.CreateTableFromStruct("parents", struct {
		ID int64 `sqlite:"id,pk"`
	}{}); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateTableFromStruct("events", testRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateTableFromStruct("events", testRecord{}); err != nil {
		t.Fatalf("idempotent create: %v", err)
	}
}

func TestTableFromStructRejectsInvalidTags(t *testing.T) {
	for _, test := range []struct {
		name string
		v any
	}{
		{"non-struct", "bad"},
		{"duplicate", struct {
			Foo int `sqlite:"same"`
			Bar int `sqlite:"same"`
		}{}},
		{"unsupported", struct{ Data map[string]any }{}},
		{"invalid-autoincrement", struct {
			ID string `sqlite:"pk,autoincrement"`
		}{}},
		{"invalid-reference", struct {
			ID int `sqlite:"references=broken"`
		}{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := TableFromStruct("items", test.v); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
