package sqlite

import (
	external "database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//goland:noinspection GoUnusedExportedFunction
func Init(dbInfo DBInfo) (DBI, error) {
	db, err := external.Open("sqlite", filepath.Join(dbPath, dbInfo.Project, dbInfo.Name+".db"))
	if err != nil {
		log.Fatalf("[InitDB] Помилка при підключенні до БД: %v", err)
		return nil, err
	}

	for _, dbTable := range dbInfo.Tables {
		var query strings.Builder
		query.WriteString(fmt.Sprintf(sqlCreateTable+" %s", dbTable.Name))
		for i, col := range dbTable.Columns {
			var c strings.Builder
			c.WriteString(fmt.Sprintf("\n\t%s %s", col.Name, col.Type))

			if col.IsPrimaryKey {
				c.WriteString(" PRIMARY KEY")
			} else {
				if col.IsNotNull {
					c.WriteString(" NOT NULL")
				}
				if col.IsUnique {
					c.WriteString(" UNIQUE")
				}
			}

			if col.Autoincrement {
				c.WriteString(" AUTOINCREMENT")
			} else if col.Default != "" {
				c.WriteString(" DEFAULT " + col.Default)
			}

			if i < len(dbTable.Columns)-1 {
				c.WriteString(",\n")
			} else {
				c.WriteString("\n)")
			}
			query.WriteString(c.String())
		}
		log.Printf("[DEBUG] %s\n", query.String())
		err = createTable(db, query.String())
		if err != nil {
			return nil, err
		}
	}
	return &dbImpl{Sqlite: db}, nil
}

func createTable(db *external.DB, query string) error {
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("[ERROR] Помилка при створенні таблиці torrent_files: %v", err)
		return err
	}
	log.Println("[INFO] Таблиця ініційована")
	return nil
}
