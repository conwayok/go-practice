package bank_test

import (
	"errors"
	"go-practice/internal/bank"
	"testing"

	"github.com/google/uuid"
)

func TestCurrency(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T, service bank.Service)
	}{
		{
			name: "create currency inserts into currencies table",
			testFunc: func(t *testing.T, service bank.Service) {
				err := service.CreateCurrency(t.Context(), bank.CreateCurrencyRequest{
					ID:       "USD",
					Decimals: 2,
				})

				if err != nil {
					t.Fatal(err)
				}

				currencies, err := service.GetCurrencies(t.Context())
				if err != nil {
					t.Fatal(err)
				}

				if len(currencies.Currencies) != 1 {
					t.Fatalf("expected 1 currency, got %d", len(currencies.Currencies))
				}

				c := currencies.Currencies[0]
				if c.ID != "USD" || c.Decimals != 2 {
					t.Fatalf("unexpected currency: %+v", c)
				}
			},
		},
		{
			name: "create currency blocks duplicates",
			testFunc: func(t *testing.T, service bank.Service) {

				request := bank.CreateCurrencyRequest{
					ID:       "USD",
					Decimals: 2,
				}

				err := service.CreateCurrency(t.Context(), request)

				if err != nil {
					t.Fatal(err)
				}

				err = service.CreateCurrency(t.Context(), request)

				if !errors.Is(err, bank.ErrCurrencyAlreadyExists) {
					t.Fatalf("expected %T, got %T (%s)", bank.ErrCurrencyAlreadyExists, err, err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := setupBankService(t)

			t.Cleanup(func() {
				service.Close()
				logTableChanges(t, "currencies")
				resetDB(t)
			})

			testCase.testFunc(t, service)
		})
	}
}

func TestCreateCurrencyValidation(t *testing.T) {
	testCases := []struct {
		name        string
		request     bank.CreateCurrencyRequest
		expectedErr error
	}{
		{
			name: "empty currency id",
			request: bank.CreateCurrencyRequest{
				ID:       "",
				Decimals: 2,
			},
			expectedErr: bank.ErrInvalidCurrencyID,
		},
		{
			name: "currency id with invalid characters",
			request: bank.CreateCurrencyRequest{
				ID:       "USD@",
				Decimals: 2,
			},
			expectedErr: bank.ErrInvalidCurrencyID,
		},
		{
			name: "negative decimal",
			request: bank.CreateCurrencyRequest{
				ID:       "USD",
				Decimals: -1,
			},
			expectedErr: bank.ErrInvalidCurrencyDecimals,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := setupBankService(t)

			t.Cleanup(func() {
				service.Close()
				logTableChanges(t, "currencies")
				resetDB(t)
			})

			err := service.CreateCurrency(t.Context(), testCase.request)

			if !errors.Is(err, testCase.expectedErr) {
				t.Fatalf("expected error %T (%s), got %T (%s)", testCase.expectedErr, testCase.expectedErr, err, err)
			}
		})
	}
}

func TestAccount(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T, service bank.Service)
	}{
		{
			name: "create account inserts into accounts table",
			testFunc: func(t *testing.T, service bank.Service) {
				createAccountRequest := bank.CreateAccountRequest{
					Name:           "test_" + uuid.NewString(),
					Currency:       "USD",
					IdempotencyKey: uuid.NewString(),
				}

				createAccountResponse, err := service.CreateAccount(t.Context(), createAccountRequest)

				if err != nil {
					t.Fatal(err)
				}

				accountInfo, err := service.GetAccountInfo(t.Context(), bank.GetAccountInfoRequest{
					ID: createAccountResponse.ID,
				})

				if err != nil {
					t.Fatal(err)
				}

				if accountInfo.AccountID != createAccountResponse.ID {
					t.Fatalf("expected account ID %v, got %v", createAccountResponse.ID, accountInfo.AccountID)
				}

				if accountInfo.AccountName != createAccountRequest.Name {
					t.Fatalf("expected account name %v, got %v", createAccountRequest.Name, accountInfo.AccountName)
				}

				if accountInfo.CurrencyID != createAccountRequest.Currency {
					t.Fatalf("expected currency %v, got %v", createAccountRequest.Currency, accountInfo.CurrencyID)
				}

				if accountInfo.CurrencyID != "USD" {
					t.Fatalf("expected currency USD, got %v", accountInfo.CurrencyID)
				}

				if accountInfo.CurrencyDecimals != 2 {
					t.Fatalf("expected currency decimals 2, got %v", accountInfo.CurrencyDecimals)
				}

				if accountInfo.BalanceAmount != 0 {
					t.Fatalf("expected balance amount 0, got %v", accountInfo.BalanceAmount)
				}
			},
		},
		{
			name: "create account checks idempotency key",
			testFunc: func(t *testing.T, service bank.Service) {
				request := bank.CreateAccountRequest{
					Name:           "Test Account",
					Currency:       "USD",
					IdempotencyKey: "123",
				}

				// create an account twice with the same idempotency key
				_, err := service.CreateAccount(t.Context(), request)

				if err != nil {
					t.Fatal(err)
				}

				_, err = service.CreateAccount(t.Context(), request)

				if !errors.Is(err, bank.ErrIdempotencyKeyExists) {
					t.Fatalf("expected %T, got %T (%s)", bank.ErrIdempotencyKeyExists, err, err)
				}
			},
		},
		{
			name: "create account returns error if currency does not exist",
			testFunc: func(t *testing.T, service bank.Service) {
				request := bank.CreateAccountRequest{
					Name:           "Test Account",
					Currency:       "ABC",
					IdempotencyKey: uuid.NewString(),
				}

				_, err := service.CreateAccount(t.Context(), request)

				if !errors.Is(err, bank.ErrCurrencyNotFound) {
					t.Fatalf("expected %v, got %v", bank.ErrCurrencyNotFound, err)
				}
			},
		},
		{
			name: "create account returns error if name is empty",
			testFunc: func(t *testing.T, service bank.Service) {
				request := bank.CreateAccountRequest{
					Name:           "",
					Currency:       "USD",
					IdempotencyKey: uuid.NewString(),
				}
				_, err := service.CreateAccount(t.Context(), request)

				if !errors.Is(err, bank.ErrAccountNameInvalid) {
					t.Fatalf("expected %v, got %v", bank.ErrAccountNameInvalid, err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := setupBankService(t)

			t.Cleanup(func() {
				service.Close()
				logTableChanges(t, "accounts", "idempotency_keys")
				resetDB(t)
			})

			createCurrency(t, service, "USD", 2)

			testCase.testFunc(t, service)
		})
	}
}

func TestDepositWithdraw(t *testing.T) {

	testCases := []struct {
		name     string
		testFunc func(t *testing.T, service bank.Service, accountID int64)
	}{
		{
			name: "deposit adds balance",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				response, err := service.Deposit(t.Context(), bank.DepositRequest{
					AccountID:      accountID,
					Amount:         100,
					IdempotencyKey: uuid.New().String(),
				})

				if err != nil {
					t.Fatal(err)
				}

				if response.BalanceBefore != 0 {
					t.Fatalf("expected balance before to be 0, got %d", response.BalanceBefore)
				}

				if response.BalanceAfter != 100 {
					t.Fatalf("expected balance after to be 100, got %d", response.BalanceAfter)
				}

				assertAccountHasBalance(t, service, accountID, 100)
				assertAccountHasTransaction(t, accountID, bank.TxTypeDeposit, 100)
			},
		},
		{
			name: "deposit returns error if idempotency key exists",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				request := bank.DepositRequest{
					AccountID:      accountID,
					Amount:         100,
					IdempotencyKey: uuid.New().String(),
				}

				_, err := service.Deposit(t.Context(), request)

				if err != nil {
					t.Fatal(err)
				}

				_, err = service.Deposit(t.Context(), request)

				if !errors.Is(err, bank.ErrIdempotencyKeyExists) {
					t.Fatalf("expected %v, got %v", bank.ErrIdempotencyKeyExists, err)
				}
			},
		},
		{
			name: "deposit returns error if account not found",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				_, err := service.Deposit(t.Context(), bank.DepositRequest{
					AccountID:      accountID + 1,
					Amount:         100,
					IdempotencyKey: uuid.New().String(),
				})

				if !errors.Is(err, bank.ErrAccountNotFound) {
					t.Fatalf("expected %v, got %v", bank.ErrAccountNotFound, err)
				}
			},
		},
		{
			name: "deposit returns error if amount is negative",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				_, err := service.Deposit(t.Context(), bank.DepositRequest{
					AccountID:      accountID,
					Amount:         -100,
					IdempotencyKey: uuid.New().String(),
				})

				if !errors.Is(err, bank.ErrInvalidTransactionAmount) {
					t.Fatalf("expected %v, got %v", bank.ErrInvalidTransactionAmount, err)
				}
			},
		},
		{
			name: "deposit returns error if amount is 0",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				_, err := service.Deposit(t.Context(), bank.DepositRequest{
					AccountID:      accountID,
					Amount:         0,
					IdempotencyKey: uuid.New().String(),
				})

				if !errors.Is(err, bank.ErrInvalidTransactionAmount) {
					t.Fatalf("expected %v, got %v", bank.ErrInvalidTransactionAmount, err)
				}
			},
		},
		{
			name: "withdraw deducts balance",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				addBalanceToAccount(t, service, accountID, 100)

				withdrawResponse, err := service.Withdraw(t.Context(), bank.WithdrawRequest{
					AccountID:      accountID,
					Amount:         10,
					IdempotencyKey: uuid.NewString(),
				})

				if err != nil {
					t.Fatal(err)
				}

				if withdrawResponse.BalanceBefore != 100 {
					t.Fatalf("expected balance before to be 100, got %d", withdrawResponse.BalanceBefore)
				}

				if withdrawResponse.BalanceAfter != 90 {
					t.Fatalf("expected balance after to be 90, got %d", withdrawResponse.BalanceAfter)
				}

				assertAccountHasBalance(t, service, accountID, 90)
				assertAccountHasTransaction(t, accountID, bank.TxTypeWithdrawal, -10)
			},
		},
		{
			name: "withdraw returns error if insufficient balance",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				addBalanceToAccount(t, service, accountID, 100)

				_, err := service.Withdraw(t.Context(), bank.WithdrawRequest{
					AccountID:      accountID,
					Amount:         100 + 1,
					IdempotencyKey: uuid.NewString(),
				})

				if !errors.Is(err, bank.ErrInsufficientBalance) {
					t.Fatalf("expected %v, got %v", bank.ErrInsufficientBalance, err)
				}

				assertAccountHasBalance(t, service, accountID, 100)
			},
		},
		{
			name: "withdraw returns error if idempotency key exists",
			testFunc: func(t *testing.T, service bank.Service, accountID int64) {
				addBalanceToAccount(t, service, accountID, 100)

				request := bank.WithdrawRequest{
					AccountID:      accountID,
					Amount:         10,
					IdempotencyKey: uuid.NewString(),
				}

				_, err := service.Withdraw(t.Context(), request)

				if err != nil {
					t.Fatal(err)
				}

				_, err = service.Withdraw(t.Context(), request)

				if !errors.Is(err, bank.ErrIdempotencyKeyExists) {
					t.Fatalf("expected %v, got %v", bank.ErrIdempotencyKeyExists, err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := setupBankService(t)

			t.Cleanup(func() {
				service.Close()
				logTableChanges(t, "transactions")
				resetDB(t)
			})

			createCurrency(t, service, "USD", 2)

			account := createAccount(t, service, "USD")

			testCase.testFunc(t, service, account)
		})
	}
}

func TestTransfer(t *testing.T) {

	initialBalance := int64(100)

	testCases := []struct {
		name     string
		testFunc func(t *testing.T, service bank.Service, accountUSD1 int64, accountUSD2 int64)
	}{
		{
			name: "transfer deducts fromAccount, credits toAccount",
			testFunc: func(t *testing.T, service bank.Service, accountUSD1 int64, accountUSD2 int64) {

				response, err := service.Transfer(t.Context(), bank.TransferRequest{
					AccountIDFrom:  accountUSD1,
					AccountIDTo:    accountUSD2,
					Amount:         10,
					IdempotencyKey: uuid.NewString(),
				})

				if err != nil {
					t.Fatal(err)
				}

				if response.AccountFromBalanceBefore != initialBalance {
					t.Fatalf("expected account from balance before to be %d, got %d", initialBalance, response.AccountFromBalanceBefore)
				}

				if response.AccountToBalanceBefore != initialBalance {
					t.Fatalf("expected account to balance before to be %d, got %d", initialBalance, response.AccountToBalanceBefore)
				}

				expectedFromBalanceAfter := initialBalance - 10
				expectedToBalanceAfter := initialBalance + 10

				if response.AccountFromBalanceAfter != expectedFromBalanceAfter {
					t.Fatalf("expected account to balance after to be %d, got %d", expectedFromBalanceAfter, response.AccountFromBalanceAfter)
				}

				if response.AccountToBalanceAfter != expectedToBalanceAfter {
					t.Fatalf("expected account to balance after to be %d, got %d", expectedToBalanceAfter, response.AccountToBalanceAfter)
				}

				assertAccountHasBalance(t, service, accountUSD1, expectedFromBalanceAfter)
				assertAccountHasBalance(t, service, accountUSD2, expectedToBalanceAfter)
				assertAccountHasTransaction(t, accountUSD1, bank.TxTypeTransfer, -10)
				assertAccountHasTransaction(t, accountUSD2, bank.TxTypeTransfer, 10)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := setupBankService(t)

			t.Cleanup(func() {
				service.Close()
				logTableChanges(t, "transactions")
				resetDB(t)
			})

			createCurrency(t, service, "USD", 2)

			accountUSD1 := createAccount(t, service, "USD")
			accountUSD2 := createAccount(t, service, "USD")

			addBalanceToAccount(t, service, accountUSD1, initialBalance)
			addBalanceToAccount(t, service, accountUSD2, initialBalance)

			testCase.testFunc(t, service, accountUSD1, accountUSD2)
		})
	}
}

func TestTransferCurrencyMismatchError(t *testing.T) {
	service := setupBankService(t)

	t.Cleanup(func() {
		service.Close()
		logTableChanges(t, "transactions")
		resetDB(t)
	})

	createCurrency(t, service, "USD", 2)
	createCurrency(t, service, "TWD", 0)

	accountUSD := createAccount(t, service, "USD")
	accountTWD := createAccount(t, service, "TWD")

	addBalanceToAccount(t, service, accountUSD, int64(100))
	addBalanceToAccount(t, service, accountTWD, int64(100))

	_, err := service.Transfer(t.Context(), bank.TransferRequest{
		AccountIDFrom:  accountUSD,
		AccountIDTo:    accountTWD,
		Amount:         10,
		IdempotencyKey: uuid.NewString(),
	})

	if !errors.Is(err, bank.ErrTransferCurrencyMismatch) {
		t.Fatalf("expected %v, got %v", bank.ErrTransferCurrencyMismatch, err)
	}
}
