package bank

import "time"

type ErrorCode int

const (
	ErrorCodeUnknown                 ErrorCode = 9000
	ErrorCodeInvalidParams           ErrorCode = 1001
	ErrorCodeInvalidOperation        ErrorCode = 1002
	ErrorCodeDuplicateIdempotencyKey ErrorCode = 1003
)

type APIErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type CreateCurrencyRequest struct {
	ID       string `json:"id"`
	Decimals int    `json:"decimals"`
}

type GetCurrenciesResponse struct {
	Currencies []CurrencyResponse `json:"currencies"`
}

type CurrencyResponse struct {
	ID       string `json:"id"`
	Decimals int    `json:"decimals"`
}

type CreateAccountRequest struct {
	Name           string `json:"name"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
}

type CreateAccountResponse struct {
	ID int64 `json:"id"`
}

type GetAccountInfoResponse struct {
	AccountID        int64     `json:"account_id"`
	AccountName      string    `json:"account_name"`
	AccountCreatedAt time.Time `json:"account_created_at"`
	CurrencyID       string    `json:"currency_id"`
	CurrencyDecimals int       `json:"currency_decimals"`
	BalanceAmount    int64     `json:"balance_amount"`
	BalanceUpdatedAt time.Time `json:"balance_updated_at"`
}

type GetAccountInfoRequest struct {
	ID int64 `json:"id"`
}

type TransferRequest struct {
	AccountIDFrom  int64  `json:"account_id_from"`
	AccountIDTo    int64  `json:"account_id_to"`
	Amount         int64  `json:"Amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type TransferResponse struct {
	AccountFromBalanceBefore int64     `json:"account_from_balance_before"`
	AccountFromBalanceAfter  int64     `json:"account_from_balance_after"`
	AccountToBalanceBefore   int64     `json:"account_to_balance_before"`
	AccountToBalanceAfter    int64     `json:"account_to_balance_after"`
	TxTime                   time.Time `json:"tx_time"`
}

type DepositRequest struct {
	AccountID      int64  `json:"account_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type DepositResponse struct {
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	TxTime        time.Time `json:"tx_time"`
}

type WithdrawRequest struct {
	AccountID      int64  `json:"account_id"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type WithdrawResponse struct {
	BalanceBefore int64     `json:"balance_before"`
	BalanceAfter  int64     `json:"balance_after"`
	TxTime        time.Time `json:"tx_time"`
}
