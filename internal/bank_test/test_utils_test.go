package bank_test

import (
	"context"
	"go-practice/internal/bank"
	"go-practice/internal/bank_migrations"
	"go-practice/internal/dbchangelog"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var dbConnStr string
var dbChangeLogManager = dbchangelog.New("__debug")

func setupBankService(t *testing.T) bank.Service {
	t.Helper()

	pool, err := pgxpool.New(t.Context(), dbConnStr)

	if err != nil {
		t.Fatal(err)
	}

	idProvider, err := bank.NewIDProvider(0)

	if err != nil {
		t.Fatal(err)
	}

	bankService, err := bank.New(
		pool,
		idProvider,
		bank.NewTimeProvider(),
	)

	if err != nil {
		t.Fatal(err)
	}

	return bankService
}

func logTableChanges(t *testing.T, tableNames ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	conn, err := pgx.Connect(ctx, dbConnStr)

	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close(ctx)

	logs, err := dbChangeLogManager.GetLogs(ctx, conn, tableNames)

	if err != nil {
		t.Fatal(err)
	}

	treeString := dbChangeLogManager.ToAsciiTreeString(logs)

	t.Logf("\n%s", treeString)
}

func resetDB(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	conn, err := pgx.Connect(ctx, dbConnStr)

	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA()")

	if err != nil {
		t.Fatal(err)
	}

	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])

	if err != nil {
		t.Fatal(err)
	}

	for _, table := range tables {
		_, err := conn.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")

		if err != nil {
			t.Fatal(err)
		}
	}
}

func createCurrency(t *testing.T, bankService bank.Service, id string, decimals int) {
	t.Helper()

	err := bankService.CreateCurrency(t.Context(), bank.CreateCurrencyRequest{
		ID:       id,
		Decimals: decimals,
	})

	if err != nil {
		t.Fatal(err)
	}
}

func assertAccountHasBalance(t *testing.T, bankService bank.Service, accountID int64, balance int64) {
	t.Helper()

	accountInfo, err := bankService.GetAccountInfo(t.Context(), bank.GetAccountInfoRequest{
		ID: accountID,
	})

	if err != nil {
		t.Fatal(err)
	}

	if accountInfo.BalanceAmount != balance {
		t.Fatalf("expected balance %d, got %d", balance, accountInfo.BalanceAmount)
	}
}

func assertAccountHasTransaction(t *testing.T, accountID int64, txType bank.TxType, amount int64) {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), dbConnStr)

	if err != nil {
		t.Fatal(err)
	}

	defer conn.Close(t.Context())

	transactions, err := bank.GetTransactionsByAccountID(t.Context(), conn, accountID)

	if err != nil {
		t.Fatal(err)
	}

	matched := false

	for _, t := range transactions {
		if t.TxType == txType && t.Amount == amount {
			matched = true
			break
		}
	}

	if !matched {
		t.Fatalf("expected account to have transaction with type %v and amount %d", txType, amount)
	}
}

func createAccount(t *testing.T, bankService bank.Service, currency string) int64 {
	t.Helper()

	response, err := bankService.CreateAccount(t.Context(), bank.CreateAccountRequest{
		Name:           "account_" + uuid.New().String(),
		Currency:       currency,
		IdempotencyKey: uuid.New().String(),
	})

	if err != nil {
		t.Fatal(err)
	}

	return response.ID
}

func addBalanceToAccount(t *testing.T, bankService bank.Service, accountID int64, amount int64) {
	t.Helper()

	_, err := bankService.Deposit(t.Context(), bank.DepositRequest{
		AccountID:      accountID,
		Amount:         amount,
		IdempotencyKey: uuid.New().String(),
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	dbName := "bankService"
	dbUser := "db_user"
	dbPassword := "db_password"

	ctx := context.Background()

	postgresContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
	)

	if err != nil {
		panic(err)
	}

	dbConnStr, err = postgresContainer.ConnectionString(ctx, "sslmode=disable")

	err = bank_migrations.Up(ctx, math.MaxUint32, dbConnStr)

	if err != nil {
		panic(err)
	}

	err = dbChangeLogManager.Setup(ctx, dbConnStr)

	if err != nil {
		panic(err)
	}

	code := m.Run()

	err = testcontainers.TerminateContainer(postgresContainer)

	if err != nil {
		panic(err)
	}

	os.Exit(code)
}
