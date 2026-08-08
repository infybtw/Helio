package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load local development settings; container environments provide variables directly.
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Error("failed to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	if err := run(db, command); err != nil {
		log.Error("migration failed", "command", command, "error", err)
		os.Exit(1)
	}

	log.Info("migration completed", "command", command)
}

func run(db *sql.DB, command string) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	version := func() (int64, error) {
		if len(os.Args) < 3 {
			return 0, fmt.Errorf("%s requires a version argument", command)
		}
		return strconv.ParseInt(os.Args[2], 10, 64)
	}

	switch command {
	case "up":
		return goose.Up(db, "migrations")
	case "up-to":
		v, err := version()
		if err != nil {
			return err
		}
		return goose.UpTo(db, "migrations", v)
	case "down":
		return goose.Down(db, "migrations")
	case "down-to":
		v, err := version()
		if err != nil {
			return err
		}
		return goose.DownTo(db, "migrations", v)
	case "status":
		return goose.Status(db, "migrations")
	case "version":
		return goose.Version(db, "migrations")
	default:
		return fmt.Errorf("unknown command: %q (supported: up, up-to, down, down-to, status, version)", command)
	}
}
