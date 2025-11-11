package bank_migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type migrationVersion struct {
	Version     int
	AppliedAt   time.Time
	Description string
}

func ensureVersionTable(ctx context.Context, conn *pgx.Conn) error {
	sql := "CREATE TABLE IF NOT EXISTS __db_migrations (version INT PRIMARY KEY, applied_at TIMESTAMP WITH TIME ZONE NOT NULL, description TEXT NOT NULL);"

	_, err := conn.Exec(ctx, sql)

	return err
}

func getMaxAppliedVersion(ctx context.Context, conn *pgx.Conn) (int, error) {
	sql := "SELECT version FROM __db_migrations ORDER BY version DESC LIMIT 1;"

	var version int

	err := conn.QueryRow(ctx, sql).Scan(&version)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}

	return version, err
}

func insertMigration(ctx context.Context, tx pgx.Tx, m migrationVersion) error {
	sql := "INSERT INTO __db_migrations (version, applied_at, description) VALUES ($1, $2, $3);"

	_, err := tx.Exec(ctx, sql, m.Version, m.AppliedAt, m.Description)

	return err
}

func deleteMigration(ctx context.Context, tx pgx.Tx, version int) error {
	sql := "DELETE FROM __db_migrations WHERE version = $1;"

	_, err := tx.Exec(ctx, sql, version)

	return err
}

func Up(ctx context.Context, targetVersion int, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)

	if err != nil {
		return err
	}

	defer conn.Close(ctx)

	err = ensureVersionTable(ctx, conn)

	if err != nil {
		return err
	}

	maxAppliedVersion, err := getMaxAppliedVersion(ctx, conn)

	if err != nil {
		return err
	}

	for _, m := range Migrations {
		if m.Version > targetVersion {
			break
		}

		if m.Version <= maxAppliedVersion {
			continue
		}

		err := pgx.BeginTxFunc(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {

			fmt.Printf("up, ver %d: %s\n", m.Version, m.Description)

			err := m.Up(ctx, tx)

			if err != nil {
				return err
			}

			err = insertMigration(ctx, tx, migrationVersion{
				Version:     m.Version,
				AppliedAt:   time.Now().UTC(),
				Description: m.Description,
			})

			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func Down(ctx context.Context, targetVersion int, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)

	if err != nil {
		return err
	}

	defer conn.Close(ctx)

	err = ensureVersionTable(ctx, conn)

	if err != nil {
		return err
	}

	maxAppliedVersion, err := getMaxAppliedVersion(ctx, conn)

	if err != nil {
		return err
	}

	for i := len(Migrations) - 1; i >= 0; i-- {
		m := Migrations[i]

		if m.Version > maxAppliedVersion {
			continue
		}

		if m.Version < targetVersion {
			break
		}

		err := pgx.BeginTxFunc(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
			fmt.Printf("down, ver %d: %s\n", m.Version, m.Description)

			err := m.Down(ctx, tx)

			if err != nil {
				return err
			}

			err = deleteMigration(ctx, tx, m.Version)

			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}
