package bank

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service we can split methods into separate interfaces if needed but keeping it simple for now.
type Service interface {
	CreateCurrency(ctx context.Context, request CreateCurrencyRequest) error
	GetCurrencies(ctx context.Context) (*GetCurrenciesResponse, error)
	CreateAccount(ctx context.Context, request CreateAccountRequest) (*CreateAccountResponse, error)
	GetAccountInfo(ctx context.Context, request GetAccountInfoRequest) (*GetAccountInfoResponse, error)
	Deposit(ctx context.Context, request DepositRequest) (*DepositResponse, error)
	Withdraw(ctx context.Context, request WithdrawRequest) (*WithdrawResponse, error)
	Transfer(ctx context.Context, request TransferRequest) (*TransferResponse, error)
	Close()
}

type service struct {
	logger       *slog.Logger
	dbPool       *pgxpool.Pool
	idProvider   IDProvider
	timeProvider TimeProvider
}

func New(dbPool *pgxpool.Pool, idProvider IDProvider, timeProvider TimeProvider) (Service, error) {
	return &service{
		dbPool:       dbPool,
		idProvider:   idProvider,
		timeProvider: timeProvider,
	}, nil
}

func (s *service) Close() {
	s.dbPool.Close()
}

func (s *service) CreateCurrency(ctx context.Context, request CreateCurrencyRequest) error {

	if !CurrencyIdRegex.MatchString(request.ID) {
		return ErrInvalidCurrencyID
	}

	if request.Decimals < 0 {
		return ErrInvalidCurrencyDecimals
	}

	currency := Currency{ID: request.ID, Decimals: request.Decimals}

	err := pgx.BeginTxFunc(ctx, s.dbPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		err := InsertCurrency(ctx, tx, currency)

		if err != nil {
			var pgErr *pgconn.PgError

			if errors.As(err, &pgErr) {
				if pgErr.Code == PostgresErrorUniqueViolation {
					return ErrCurrencyAlreadyExists
				}
			}

			return fmt.Errorf("insert currency failed: %w", err)
		}

		return nil
	})

	return err
}

func (s *service) GetCurrencies(ctx context.Context) (*GetCurrenciesResponse, error) {
	currencies, err := GetCurrencies(ctx, s.dbPool)

	if err != nil {
		return nil, fmt.Errorf("get currencies from DB failed: %w", err)
	}

	currencyResponses := make([]CurrencyResponse, len(currencies))

	for i, currency := range currencies {
		currencyResponses[i] = CurrencyResponse{
			ID:       currency.ID,
			Decimals: currency.Decimals,
		}
	}

	return &GetCurrenciesResponse{Currencies: currencyResponses}, nil
}

func (s *service) CreateAccount(ctx context.Context, request CreateAccountRequest) (*CreateAccountResponse, error) {

	if request.Name == "" {
		return nil, ErrAccountNameInvalid
	}

	exists, err := CurrencyExists(ctx, s.dbPool, request.Currency)

	if err != nil {
		return nil, fmt.Errorf("create account check currency exists failed: %w", err)
	}

	if !exists {
		return nil, ErrCurrencyNotFound
	}

	account := Account{
		ID:         s.idProvider.NextID(),
		Name:       request.Name,
		CurrencyID: request.Currency,
		CreatedAt:  s.timeProvider.NowUTC(),
	}

	err = pgx.BeginTxFunc(ctx, s.dbPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rowsChanged, err := InsertIdempotencyKey(ctx, tx, IdempotencyKey{
			Key:       request.IdempotencyKey,
			CreatedAt: time.Now().UTC(),
		})

		if err != nil {
			return err
		}

		if rowsChanged == 0 {
			return ErrIdempotencyKeyExists
		}

		err = InsertAccount(ctx, tx, account)

		if err != nil {
			return err
		}

		return InsertZeroBalance(ctx, tx, account.ID, account.CreatedAt)
	})

	if err != nil {
		return nil, err
	}

	return &CreateAccountResponse{
		ID: account.ID,
	}, nil
}

func (s *service) GetAccountInfo(ctx context.Context, request GetAccountInfoRequest) (*GetAccountInfoResponse, error) {

	accountInfo, err := GetAccountInfoByID(ctx, s.dbPool, request.ID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}

		return nil, err
	}

	return &GetAccountInfoResponse{
		AccountID:        accountInfo.AccountID,
		AccountName:      accountInfo.AccountName,
		AccountCreatedAt: accountInfo.AccountCreatedAt,
		CurrencyID:       accountInfo.CurrencyID,
		CurrencyDecimals: accountInfo.CurrencyDecimals,
		BalanceAmount:    accountInfo.BalanceAmount,
		BalanceUpdatedAt: accountInfo.BalanceUpdatedAt,
	}, nil
}

func (s *service) Deposit(ctx context.Context, request DepositRequest) (*DepositResponse, error) {

	if request.Amount <= 0 {
		return nil, ErrInvalidTransactionAmount
	}

	var balanceBefore, balanceAfter int64

	utc := s.timeProvider.NowUTC()

	err := pgx.BeginTxFunc(ctx, s.dbPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rowsChanged, err := InsertIdempotencyKey(ctx, tx, IdempotencyKey{
			Key:       request.IdempotencyKey,
			CreatedAt: utc,
		})

		if err != nil {
			return fmt.Errorf("deposit insert idempotency key failed: %w", err)
		}

		if rowsChanged == 0 {
			return ErrIdempotencyKeyExists
		}

		balance, err := GetBalanceWithLock(ctx, tx, request.AccountID)

		if err != nil {
			return fmt.Errorf("deposit get balance failed: %w", err)
		}

		if balance == nil {
			return ErrAccountNotFound
		}

		balanceBefore = balance.Amount

		balance.Amount += request.Amount
		balance.UpdatedAt = utc

		balanceAfter = balance.Amount

		err = UpdateBalance(ctx, tx, balance)

		if err != nil {
			return fmt.Errorf("deposit update balance failed: %w", err)
		}

		err = InsertTransaction(ctx, tx, Transaction{
			ID:        s.idProvider.NextID(),
			AccountID: request.AccountID,
			TxType:    TxTypeDeposit,
			Amount:    request.Amount,
			Timestamp: utc,
			ParentID:  nil,
		})

		if err != nil {
			return fmt.Errorf("deposit insert transaction failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &DepositResponse{
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		TxTime:        utc,
	}, nil
}

func (s *service) Withdraw(ctx context.Context, request WithdrawRequest) (*WithdrawResponse, error) {

	if request.Amount <= 0 {
		return nil, ErrInvalidTransactionAmount
	}

	var balanceBefore, balanceAfter int64

	utc := s.timeProvider.NowUTC()

	err := pgx.BeginTxFunc(ctx, s.dbPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rowsChanged, err := InsertIdempotencyKey(ctx, tx, IdempotencyKey{
			Key:       request.IdempotencyKey,
			CreatedAt: utc,
		})

		if err != nil {
			return fmt.Errorf("withdraw insert idempotency key failed: %w", err)
		}

		if rowsChanged == 0 {
			return ErrIdempotencyKeyExists
		}

		balance, err := GetBalanceWithLock(ctx, tx, request.AccountID)

		if err != nil {
			return fmt.Errorf("withdraw get balance failed: %w", err)
		}

		if balance == nil {
			return ErrAccountNotFound
		}

		balanceBefore = balance.Amount

		balance.Amount -= request.Amount

		if balance.Amount < 0 {
			return ErrInsufficientBalance
		}

		balance.UpdatedAt = utc

		balanceAfter = balance.Amount

		err = UpdateBalance(ctx, tx, balance)

		if err != nil {
			return fmt.Errorf("withdraw update balance failed: %w", err)
		}

		err = InsertTransaction(ctx, tx, Transaction{
			ID:        s.idProvider.NextID(),
			AccountID: request.AccountID,
			TxType:    TxTypeWithdrawal,
			Amount:    -request.Amount,
			Timestamp: utc,
			ParentID:  nil,
		})

		if err != nil {
			return fmt.Errorf("withdraw insert transaction failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &WithdrawResponse{
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		TxTime:        utc,
	}, nil
}

func (s *service) Transfer(ctx context.Context, request TransferRequest) (*TransferResponse, error) {

	if request.Amount == 0 {
		return nil, ErrInvalidTransactionAmount
	}

	fromAccountCurrencyID, err := GetAccountCurrencyID(ctx, s.dbPool, request.AccountIDFrom)

	if err != nil {
		return nil, fmt.Errorf("transfer get currency ID failed for fromAccount: %w", err)
	}

	if fromAccountCurrencyID == "" {
		return nil, ErrTransferFromAccountNotFound
	}

	toAccountCurrencyID, err := GetAccountCurrencyID(ctx, s.dbPool, request.AccountIDTo)

	if err != nil {
		return nil, fmt.Errorf("transfer get currency ID failed for toAccount: %w", err)
	}

	if toAccountCurrencyID == "" {
		return nil, ErrTransferToAccountNotFound
	}

	if fromAccountCurrencyID != toAccountCurrencyID {
		return nil, ErrTransferCurrencyMismatch
	}

	var fromBalanceBefore, toBalanceBefore int64
	var fromBalanceAfter, toBalanceAfter int64

	utc := s.timeProvider.NowUTC()

	err = pgx.BeginTxFunc(ctx, s.dbPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		rowsChanged, err := InsertIdempotencyKey(ctx, tx, IdempotencyKey{
			Key:       request.IdempotencyKey,
			CreatedAt: utc,
		})

		if err != nil {
			return err
		}

		if rowsChanged == 0 {
			return ErrIdempotencyKeyExists
		}

		fromBalance, err := GetBalanceWithLock(ctx, tx, request.AccountIDFrom)

		if err != nil {
			return err
		}

		if fromBalance == nil {
			return fmt.Errorf("cannot find balance for account %d", request.AccountIDFrom)
		}

		if fromBalance.Amount < request.Amount {
			return ErrInsufficientBalance
		}

		toBalance, err := GetBalanceWithLock(ctx, tx, request.AccountIDTo)

		if err != nil {
			return err
		}

		if toBalance == nil {
			return fmt.Errorf("cannot find balance for account %d", request.AccountIDTo)
		}

		fromBalanceBefore = fromBalance.Amount
		toBalanceBefore = toBalance.Amount

		fromBalance.Amount -= request.Amount
		toBalance.Amount += request.Amount

		fromBalanceAfter = fromBalance.Amount
		toBalanceAfter = toBalance.Amount

		err = UpdateBalance(ctx, tx, fromBalance)

		if err != nil {
			return err
		}

		err = UpdateBalance(ctx, tx, toBalance)

		if err != nil {
			return err
		}

		debitTx := Transaction{
			ID:        s.idProvider.NextID(),
			AccountID: request.AccountIDFrom,
			TxType:    TxTypeTransfer,
			Amount:    -request.Amount,
			Timestamp: utc,
			ParentID:  nil,
		}

		err = InsertTransaction(ctx, tx, debitTx)

		if err != nil {
			return err
		}

		creditTx := Transaction{
			ID:        s.idProvider.NextID(),
			AccountID: request.AccountIDTo,
			TxType:    TxTypeTransfer,
			Amount:    request.Amount,
			Timestamp: utc,
			ParentID:  &debitTx.ID,
		}

		err = InsertTransaction(ctx, tx, creditTx)

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &TransferResponse{
		AccountFromBalanceBefore: fromBalanceBefore,
		AccountFromBalanceAfter:  fromBalanceAfter,
		AccountToBalanceBefore:   toBalanceBefore,
		AccountToBalanceAfter:    toBalanceAfter,
		TxTime:                   utc,
	}, nil
}
