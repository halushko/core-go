# SQLite struct schema

The existing `CreateTable(Table)` API remains unchanged. The new
`TableFromStruct(name, model)` derives a `Table` from exported Go fields,
and `CreateTableFromStruct(name, model)` uses the existing SQL builders.

```go
type PowerEvent struct {
    ID        int64     `sqlite:"id,pk,autoincrement"`
    DeviceID  string    `sqlite:"device_id,notnull,index"`
    CreatedAt time.Time `sqlite:"created_at,notnull"`
    Voltage   float64   `sqlite:"voltage"`
    Comment   *string   `sqlite:"comment"`
}

table, err := sqlite.TableFromStruct("power_events", PowerEvent{})
if err != nil {
    return err
}
err = client.CreateTable(table)
// Or: err = client.CreateTableFromStruct("power_events", PowerEvent{})
```

Supported tags: `name` (or first bare token), `type`, `pk`,
`autoincrement`, `notnull`, `unique`, `index[=name]`,
`unique_index[=name]`, `unique_group=name`, `default=SQL`,
`check=SQL`, `references=table(column)`, `ondelete=ACTION`,
`onupdate=ACTION`; `sqlite:"-"` skips a field.

Go strings map to TEXT; bool, integer types to INTEGER; floats to REAL;
`[]byte` to BLOB; `time.Time` to DATETIME; pointers retain underlying
types. Unknown Go types fail unless `type=...` is explicit.

This is schema **creation**, not migrations: existing tables are left intact
by `CREATE TABLE IF NOT EXISTS`. SQL expressions in `default` and `check`
tags are trusted source-code values, not user input.
