package main

import (
	"context"
	"go-practice/internal/bank_migrations"
	"log"
	"os"
)

func main() {
	dsn := os.Getenv("DB_URL")

	if dsn == "" {
		log.Fatal("DB_URL is not set")
	}

	err := bank_migrations.Up(context.Background(), 999, dsn)

	if err != nil {
		log.Fatal(err)
	}
}
