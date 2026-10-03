package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"2026_2_PinPals/internal/config"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		log.Fatalf(
			"usage: %s <up|seed>",
			filepath.Base(os.Args[0]),
		)
	}

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

	var dir string

	switch os.Args[1] {
	case "up":
		dir = "migrations"

	case "seed":
		dir = "seeds"

	default:
		log.Fatalf(
			"unknown command: %q (expected \"up\" or \"seed\")",
			os.Args[1],
		)
	}

	if err := applySQLFiles(ctx, conn, dir); err != nil {
		log.Fatalf("%s failed: %v", os.Args[1], err)
	}
}

func applySQLFiles(
	ctx context.Context,
	conn *pgx.Conn,
	dir string,
) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", dir, err)
	}

	var files []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.EqualFold(filepath.Ext(entry.Name()), ".sql") {
			files = append(
				files,
				filepath.Join(dir, entry.Name()),
			)
		}
	}

	sort.Strings(files)

	if len(files) == 0 {
		return fmt.Errorf("no .sql files found in %q", dir)
	}

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}

		if err := execInTx(ctx, conn, string(content)); err != nil {
			return fmt.Errorf("apply %q: %w", path, err)
		}

		fmt.Printf("applied: %s\n", path)
	}

	return nil
}

func execInTx(
	ctx context.Context,
	conn *pgx.Conn,
	sql string,
) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("execute sql: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
