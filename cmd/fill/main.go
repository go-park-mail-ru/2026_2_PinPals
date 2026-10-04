package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"2026_2_PinPals/internal/config"
)

const seedFile = "seeds/insert.sql"

func main() {
	_ = godotenv.Load()

	cfg := config.MustLoad()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Minute,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer conn.Close(ctx)

	if err := fillDatabase(ctx, conn); err != nil {
		log.Fatalf("fill database: %v", err)
	}

	log.Println("database filled successfully")
}

func fillDatabase(ctx context.Context, conn *pgx.Conn) error {
	content, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("read seed file %q: %w", seedFile, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("execute seed file: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
