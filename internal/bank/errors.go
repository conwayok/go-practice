package bank

import (
	"errors"
)

var ErrInvalidCurrencyID = errors.New("invalid currency ID")
var ErrInvalidCurrencyDecimals = errors.New("invalid currency decimals")
var ErrCurrencyAlreadyExists = errors.New("currency already exists")
var ErrIdempotencyKeyExists = errors.New("idempotency key already exists")
var ErrTransferFromAccountNotFound = errors.New("transfer from account not found")
var ErrTransferToAccountNotFound = errors.New("transfer to account not found")
var ErrInsufficientBalance = errors.New("insufficient balance")
var ErrAccountNotFound = errors.New("deposit account not found")
var ErrInvalidTransactionAmount = errors.New("invalid deposit amount")
var ErrCurrencyNotFound = errors.New("create account currency not found")
var ErrAccountNameInvalid = errors.New("account name invalid")
var ErrTransferCurrencyMismatch = errors.New("transfer currency mismatch")
