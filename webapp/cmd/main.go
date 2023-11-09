package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func addIndexToDB(dbPath string, indexSQL string) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(indexSQL)
	if err != nil {
		log.Fatalf("Failed to add index: %v", err)
	}
}

func main() {
	tenantDBDir := "../../initial_data"
	sqls := []string{
		"CREATE INDEX IF NOT EXISTS idx_competition_on_tenant ON competition(tenant_id, created_at);",
		"CREATE INDEX IF NOT EXISTS idx_player_score_on_tenant_comp_row ON player_score(tenant_id, competition_id, row_num);",
		"CREATE INDEX IF NOT EXISTS idx_player_score_on_tenant_comp_player ON player_score(tenant_id, competition_id, player_id);",
		"CREATE INDEX IF NOT EXISTS idx_player_score_on_tenant_comp ON player_score(tenant_id, competition_id);",
	}

	err := filepath.Walk(tenantDBDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".db") {
			for _, indexSQL := range sqls {
				addIndexToDB(path, indexSQL)
				fmt.Printf("Added index to %s\n", path)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the directory: %v", err)
	}
}
