package bank_migrations

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, tx pgx.Tx) error
	Down        func(ctx context.Context, tx pgx.Tx) error
}

func createCurrencies(ctx context.Context, tx pgx.Tx) error {
	sql := `
	CREATE TABLE currencies (
		id       TEXT PRIMARY KEY,
		decimals INT NOT NULL
	);`
	_, err := tx.Exec(ctx, sql)
	return err
}

func dropCurrencies(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DROP TABLE currencies;`)
	return err
}

func createAccounts(ctx context.Context, tx pgx.Tx) error {
	sql := `
	CREATE TABLE accounts (
		id          BIGINT PRIMARY KEY,
		name        TEXT NOT NULL,
		currency_id TEXT NOT NULL REFERENCES currencies (id),
		created_at  TIMESTAMPTZ NOT NULL
	);`
	_, err := tx.Exec(ctx, sql)
	return err
}

func dropAccounts(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DROP TABLE accounts;`)
	return err
}

// --- balances ---
func createBalances(ctx context.Context, tx pgx.Tx) error {
	sql := `
	CREATE TABLE balances (
		account_id BIGINT PRIMARY KEY REFERENCES accounts (id),
		amount     BIGINT NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	);`
	_, err := tx.Exec(ctx, sql)
	return err
}

func dropBalances(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DROP TABLE balances;`)
	return err
}

// --- transactions ---
func createTransactions(ctx context.Context, tx pgx.Tx) error {
	sql := `
	CREATE TABLE transactions (
		id         BIGINT PRIMARY KEY,
		account_id BIGINT NOT NULL REFERENCES accounts (id),
		tx_type       INT NOT NULL,
		amount     BIGINT NOT NULL,
		timestamp  TIMESTAMPTZ NOT NULL,
		parent_id  BIGINT REFERENCES transactions (id)
	);`
	_, err := tx.Exec(ctx, sql)
	return err
}

func dropTransactions(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DROP TABLE transactions;`)
	return err
}

func createIdempotencyKeys(ctx context.Context, tx pgx.Tx) error {
	sql := `
	CREATE TABLE idempotency_keys (
		key TEXT PRIMARY KEY,
		created_at      TIMESTAMP WITH TIME ZONE NOT NULL
	);
	CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys (created_at);`

	_, err := tx.Exec(ctx, sql)
	return err
}

func dropIdempotencyKeys(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DROP TABLE idempotency_keys;`)
	return err
}

var Migrations = []Migration{
	{Version: 1, Description: "Create currencies table", Up: createCurrencies, Down: dropCurrencies},
	{Version: 2, Description: "Create accounts table", Up: createAccounts, Down: dropAccounts},
	{Version: 3, Description: "Create balances table", Up: createBalances, Down: dropBalances},
	{Version: 4, Description: "Create transactions table", Up: createTransactions, Down: dropTransactions},
	{Version: 5, Description: "Create idempotency_keys table", Up: createIdempotencyKeys, Down: dropIdempotencyKeys},
}
