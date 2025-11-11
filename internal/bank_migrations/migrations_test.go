package bank_migrations

import (
	"context"
	"log"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestDbMigrations(t *testing.T) {
	ctx := context.Background()

	dbName := "bank"
	dbUser := "db_user"
	dbPassword := "db_password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
	)

	defer func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()

	if err != nil {
		t.Fatal(err)
	}

	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	err = Up(ctx, 999, dsn)

	if err != nil {
		t.Fatal(err)
	}

	err = Down(ctx, 0, dsn)

	if err != nil {
		t.Fatal(err)
	}
}
