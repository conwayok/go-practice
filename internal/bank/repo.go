package bank

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	PostgresErrorUniqueViolation = "23505"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func InsertCurrency(ctx context.Context, dbTx DBTX, currency Currency) error {
	sql := "INSERT INTO currencies (id, decimals) VALUES ($1, $2)"

	_, err := dbTx.Exec(ctx, sql, currency.ID, currency.Decimals)

	if err != nil {
		return err
	}

	return nil
}

func GetCurrencies(ctx context.Context, dbtx DBTX) ([]Currency, error) {
	sql := "SELECT id, decimals FROM currencies"

	rows, err := dbtx.Query(ctx, sql)

	if err != nil {
		return nil, err
	}

	collectRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[Currency])

	return collectRows, err
}

func CurrencyExists(ctx context.Context, dbtx DBTX, id string) (bool, error) {
	sql := "SELECT EXISTS(SELECT 1 FROM currencies WHERE id = $1)"

	var exists bool

	row := dbtx.QueryRow(ctx, sql, id)

	err := row.Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}

func InsertAccount(ctx context.Context, dbtx DBTX, account Account) error {
	sql := "INSERT INTO accounts (id, name, currency_id, created_at) VALUES ($1, $2, $3, $4)"

	_, err := dbtx.Exec(ctx, sql, account.ID, account.Name, account.CurrencyID, account.CreatedAt)

	if err != nil {
		return err
	}

	return nil
}

func InsertZeroBalance(ctx context.Context, dbtx DBTX, accountID int64, updatedAt time.Time) error {
	sql := "INSERT INTO balances (account_id, Amount, updated_at) VALUES ($1, $2, $3)"

	_, err := dbtx.Exec(ctx, sql, accountID, 0, updatedAt)

	return err
}

func GetAccountInfoByID(ctx context.Context, dbtx DBTX, id int64) (*AccountInfo, error) {

	sql := `
SELECT a.id          AS account_id,
       a.name        AS account_name,
       a.created_at  AS account_created_at,
       a.currency_id AS currency_id,
       c.decimals    AS currency_decimals,
       b.Amount      AS balance_amount,
	   b.updated_at  AS balance_updated_at
FROM accounts a
         JOIN balances b ON a.id = b.account_id
         JOIN currencies c ON a.currency_id = c.id
WHERE a.id = $1;
`

	rows, err := dbtx.Query(ctx, sql, id)

	if err != nil {
		return nil, err
	}

	accountInfo, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[AccountInfo])

	if err != nil {
		return nil, err
	}

	return &accountInfo, nil
}

func GetAccountCurrencyID(ctx context.Context, dbtx DBTX, accountID int64) (string, error) {
	sql := "SELECT currency_id FROM accounts WHERE id = $1"

	var currencyID string

	row := dbtx.QueryRow(ctx, sql, accountID)

	err := row.Scan(&currencyID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}

		return "", err
	}

	return currencyID, nil
}

func InsertIdempotencyKey(ctx context.Context, dbtx DBTX, idempotencyKey IdempotencyKey) (int64, error) {
	sql := "INSERT INTO idempotency_keys (key, created_at) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING;"

	exec, err := dbtx.Exec(ctx, sql, idempotencyKey.Key, idempotencyKey.CreatedAt)

	if err != nil {
		return 0, err
	}

	return exec.RowsAffected(), nil
}

func GetBalanceWithLock(ctx context.Context, dbtx DBTX, accountID int64) (*Balance, error) {
	sql := "SELECT account_id, Amount, updated_at FROM balances WHERE account_id = $1 FOR UPDATE"

	var balance Balance
	row := dbtx.QueryRow(ctx, sql, accountID)

	err := row.Scan(&balance.AccountID, &balance.Amount, &balance.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &balance, nil
}

func UpdateBalance(ctx context.Context, dbtx DBTX, balance *Balance) error {
	sql := "UPDATE balances SET Amount = $1, updated_at = $2 WHERE account_id = $3"

	_, err := dbtx.Exec(ctx, sql, balance.Amount, balance.UpdatedAt, balance.AccountID)

	return err
}

func InsertTransaction(ctx context.Context, dbtx DBTX, txn Transaction) error {
	sql := "INSERT INTO transactions (id, account_id, tx_type, Amount, timestamp, parent_id) VALUES ($1, $2, $3, $4, $5, $6)"

	_, err := dbtx.Exec(ctx, sql, txn.ID, txn.AccountID, txn.TxType, txn.Amount, txn.Timestamp, txn.ParentID)

	return err
}

func GetTransactionsByAccountID(ctx context.Context, dbtx DBTX, accountID int64) ([]Transaction, error) {
	sql := "SELECT id, account_id, tx_type, amount, timestamp, parent_id FROM transactions WHERE account_id = $1"

	rows, err := dbtx.Query(ctx, sql, accountID)

	if err != nil {
		return make([]Transaction, 0), err
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[Transaction])

	if err != nil {
		return make([]Transaction, 0), err
	}

	return transactions, nil
}
