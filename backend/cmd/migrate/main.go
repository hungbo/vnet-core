package main

import (
	"log"

	"github.com/vnet/core/internal/config"
	"github.com/vnet/core/internal/database"
)

type migration struct {
	name string
	sql  string
}

var migrations = []migration{
	{name: "create_unaccent_extension", sql: "CREATE EXTENSION IF NOT EXISTS unaccent"},
	{name: "drop_products_category_id_not_null", sql: "ALTER TABLE products ALTER COLUMN category_id DROP NOT NULL"},
	{name: "drop_product_options_price_adjust", sql: "ALTER TABLE product_options DROP COLUMN IF EXISTS price_adjust"},
}

func main() {
	cfg := config.Load()
	db := database.Init(&cfg.Database)

	db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)

	for _, m := range migrations {
		var count int64
		db.Table("schema_migrations").Where("name = ?", m.name).Count(&count)
		if count > 0 {
			log.Printf("SKIP %s (already applied)", m.name)
			continue
		}
		if err := db.Exec(m.sql).Error; err != nil {
			log.Fatalf("FAIL %s: %v", m.name, err)
		}
		db.Exec("INSERT INTO schema_migrations (name) VALUES (?)", m.name)
		log.Printf("OK   %s", m.name)
	}
}
