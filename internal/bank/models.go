package bank

import (
	"regexp"
	"time"
)

var CurrencyIdRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

type Currency struct {
	ID       string `db:"id"`
	Decimals int    `db:"decimals"`
}

type Account struct {
	ID         int64     `db:"id"`
	Name       string    `db:"name"`
	CurrencyID string    `db:"currency_id"`
	CreatedAt  time.Time `db:"created_at"`
}

type Balance struct {
	AccountID int64     `db:"account_id"`
	Amount    int64     `db:"Amount"`
	UpdatedAt time.Time `db:"updated_at"`
}

type AccountInfo struct {
	AccountID        int64     `db:"account_id"`
	AccountName      string    `db:"account_name"`
	AccountCreatedAt time.Time `db:"account_created_at"`
	CurrencyID       string    `db:"currency_id"`
	CurrencyDecimals int       `db:"currency_decimals"`
	BalanceAmount    int64     `db:"balance_amount"`
	BalanceUpdatedAt time.Time `db:"balance_updated_at"`
}

type IdempotencyKey struct {
	Key       string    `db:"key"`
	CreatedAt time.Time `db:"created_at"`
}

type TxType int

const (
	TxTypeDeposit    TxType = 1
	TxTypeWithdrawal TxType = 2
	TxTypeTransfer   TxType = 3
)

type Transaction struct {
	ID        int64     `db:"id"`
	AccountID int64     `db:"account_id"`
	TxType    TxType    `db:"tx_type"`
	Amount    int64     `db:"amount"`
	Timestamp time.Time `db:"timestamp"`
	ParentID  *int64    `db:"parent_id"`
}
