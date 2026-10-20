package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"tripledger/config"
	"tripledger/database"
)

func main() {
	cfg := config.LoadConfig()

	fmt.Printf("[Test DB Connection] Connecting to MySQL at %s:%s with user '%s' on database '%s'...\n",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBName)

	db, err := database.InitDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "SHOW TABLES;")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED to query tables: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	fmt.Println("SUCCESS: Connected to database successfully!")
	fmt.Printf("Verified existing tables in %s: %v\n", cfg.DBName, tables)
}
