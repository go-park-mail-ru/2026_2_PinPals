package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
	"github.com/joho/godotenv"

	"2026_2_PinPals/internal/config"
)

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

	migrator, err := migrate.NewMigrator(
		ctx,
		conn,
		"public.schema_version",
	)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}

	migrator.OnStart = func(
		sequence int32,
		name string,
		direction string,
		_ string,
	) {
		log.Printf(
			"migrating %s: %d %s",
			direction,
			sequence,
			name,
		)
	}

	if err := migrator.LoadMigrations(os.DirFS("migrations")); err != nil {
		log.Fatalf("load migrations: %v", err)
	}

	if err := migrator.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Println("migrations completed successfully")
}
