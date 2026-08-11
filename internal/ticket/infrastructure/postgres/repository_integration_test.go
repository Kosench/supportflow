package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpostgres "github.com/Kosench/supportflow/internal/platform/postgres"
	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
	ticketpostgres "github.com/Kosench/supportflow/internal/ticket/infrastructure/postgres"
)

const vpnCategoryID = "019d0000-0000-7000-8000-000000000001"

func TestTicketRepositoryCreateAndGet(t *testing.T) {
	pool := openTestPool(t)
	tx := beginTestTransaction(t, pool)

	requesterID := mustNewUserID(t)
	insertTestUser(t, tx, requesterID, "USER")

	repository := ticketpostgres.NewTicketRepository(tx)
	original := newTestTicket(t, requesterID)

	if err := repository.Create(context.Background(), original); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	loaded, err := repository.GetByID(
		context.Background(),
		original.Snapshot().ID,
	)
	if err != nil {
		t.Fatalf("GetByID() returned error: %v", err)
	}

	got := loaded.Snapshot()
	want := original.Snapshot()

	if got.ID != want.ID {
		t.Errorf("unexpected ID: got %s want %s", got.ID, want.ID)
	}

	if got.Status != want.Status {
		t.Errorf(
			"unexpected status: got %s want %s",
			got.Status,
			want.Status,
		)
	}

	if got.Version != want.Version {
		t.Errorf(
			"unexpected version: got %d want %d",
			got.Version,
			want.Version,
		)
	}

	if len(loaded.Events()) != 0 {
		t.Error("restored ticket must not contain domain events")
	}
}

func TestTicketRepositoryUpdatesSeveralChangesAtOnce(t *testing.T) {
	pool := openTestPool(t)
	tx := beginTestTransaction(t, pool)
	ctx := context.Background()

	requesterID := mustNewUserID(t)
	operatorID := mustNewUserID(t)
	insertTestUser(t, tx, requesterID, "USER")
	insertTestUser(t, tx, operatorID, "OPERATOR")

	repository := ticketpostgres.NewTicketRepository(tx)
	original := newTestTicket(t, requesterID)
	mustSucceed(t, repository.Create(ctx, original))

	loaded, err := repository.GetByID(ctx, original.Snapshot().ID)
	mustSucceed(t, err)

	expectedVersion := loaded.Snapshot().Version
	now := loaded.Snapshot().CreatedAt
	mustSucceed(t, loaded.Assign(operatorID, now.Add(time.Minute)))
	mustSucceed(t, loaded.StartProgress(now.Add(2*time.Minute)))
	mustSucceed(t, repository.Update(
		ctx,
		loaded,
		expectedVersion,
	))

	stored, err := repository.GetByID(ctx, original.Snapshot().ID)
	mustSucceed(t, err)

	if stored.Snapshot().Status != domain.StatusInProgress {
		t.Errorf(
			"unexpected status: %s",
			stored.Snapshot().Status,
		)
	}

	if stored.Snapshot().Version != expectedVersion+2 {
		t.Errorf(
			"unexpected version: got %d want %d",
			stored.Snapshot().Version,
			expectedVersion+2,
		)
	}
}

func TestTicketRepositoryDetectsVersionConflict(t *testing.T) {
	pool := openTestPool(t)
	tx := beginTestTransaction(t, pool)
	ctx := context.Background()

	requesterID := mustNewUserID(t)
	firstOperatorID := mustNewUserID(t)
	secondOperatorID := mustNewUserID(t)

	insertTestUser(t, tx, requesterID, "USER")
	insertTestUser(t, tx, firstOperatorID, "OPERATOR")
	insertTestUser(t, tx, secondOperatorID, "OPERATOR")

	repository := ticketpostgres.NewTicketRepository(tx)
	original := newTestTicket(t, requesterID)
	mustSucceed(t, repository.Create(ctx, original))

	firstCopy, err := repository.GetByID(ctx, original.Snapshot().ID)
	mustSucceed(t, err)

	secondCopy, err := repository.GetByID(ctx, original.Snapshot().ID)
	mustSucceed(t, err)

	expectedVersion := firstCopy.Snapshot().Version
	now := original.Snapshot().CreatedAt
	mustSucceed(t, firstCopy.Assign(
		firstOperatorID,
		now.Add(time.Minute),
	))
	mustSucceed(t, repository.Update(
		ctx,
		firstCopy,
		expectedVersion,
	))

	mustSucceed(t, secondCopy.Assign(
		secondOperatorID,
		now.Add(2*time.Minute),
	))
	err = repository.Update(
		ctx,
		secondCopy,
		expectedVersion,
	)

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	var conflict ports.VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected VersionConflictError, got %T", err)
	}

	if conflict.Expected != 1 || conflict.Actual != 2 {
		t.Errorf(
			"unexpected versions: expected=%d actual=%d",
			conflict.Expected,
			conflict.Actual,
		)
	}

	stored, err := repository.GetByID(ctx, original.Snapshot().ID)
	mustSucceed(t, err)

	if stored.Snapshot().AssigneeID == nil {
		t.Fatal("stored ticket has no assignee")
	}

	if *stored.Snapshot().AssigneeID != firstOperatorID {
		t.Error("conflicting update overwrote stored assignee")
	}
}

func TestSLAPolicyRepositoryLoadsSeed(t *testing.T) {
	pool := openTestPool(t)
	repository := ticketpostgres.NewSLAPolicyRepository(pool)

	target, err := repository.GetByPriority(
		context.Background(),
		domain.PriorityNormal,
	)
	if err != nil {
		t.Fatalf("GetByPriority() returned error: %v", err)
	}

	if target.FirstResponse != 2*time.Hour {
		t.Errorf(
			"unexpected first response: %s",
			target.FirstResponse,
		)
	}

	if target.Resolution != 24*time.Hour {
		t.Errorf("unexpected resolution: %s", target.Resolution)
	}
}

func TestUnitOfWorkRollsBackCallbackError(t *testing.T) {
	pool := openTestPool(t)
	ctx := context.Background()
	requesterID := mustNewUserID(t)

	insertTestUser(t, pool, requesterID, "USER")
	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM ticket.users WHERE id = $1`,
			requesterID.String(),
		)
	})

	ticket := newTestTicket(t, requesterID)
	expectedError := errors.New("stop transaction")
	unit := ticketpostgres.NewUnitOfWork(pool)

	err := unit.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			if err := repositories.Tickets.Create(ctx, ticket); err != nil {
				return err
			}

			return expectedError
		},
	)
	if !errors.Is(err, expectedError) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	repository := ticketpostgres.NewTicketRepository(pool)
	_, err = repository.GetByID(ctx, ticket.Snapshot().ID)
	if !errors.Is(err, ports.ErrTicketNotFound) {
		t.Fatalf("ticket was not rolled back: %v", err)
	}
}

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	cfg := platformpostgres.DefaultConfig(
		databaseURL,
		"ticket-repository-integration-test",
	)
	cfg.MaxConns = 4
	cfg.MinConns = 0

	pool, err := platformpostgres.Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open PostgreSQL pool: %v", err)
	}
	t.Cleanup(pool.Close)

	var schemaReady bool
	err = pool.QueryRow(
		ctx,
		`SELECT to_regclass('ticket.tickets') IS NOT NULL`,
	).Scan(&schemaReady)
	if err != nil {
		t.Fatalf("check test schema: %v", err)
	}
	if !schemaReady {
		t.Fatal("ticket migrations are not applied")
	}

	return pool
}

func beginTestTransaction(
	t *testing.T,
	pool *pgxpool.Pool,
) pgx.Tx {
	t.Helper()

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin test transaction: %v", err)
	}

	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})

	return tx
}

func insertTestUser(
	t *testing.T,
	db interface {
		Exec(
			context.Context,
			string,
			...any,
		) (pgconn.CommandTag, error)
	},
	userID domain.UserID,
	role string,
) {
	t.Helper()

	_, err := db.Exec(
		context.Background(),
		`
            INSERT INTO ticket.users (
                id,
                email,
                password_hash,
                display_name,
                role
            )
            VALUES ($1, $2, $3, $4, $5)
        `,
		userID.String(),
		fmt.Sprintf("%s@example.com", userID),
		"$integration-test-hash",
		"Integration Test User",
		role,
	)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
}

func newTestTicket(
	t *testing.T,
	requesterID domain.UserID,
) *domain.Ticket {
	t.Helper()

	ticketID, err := domain.NewTicketID()
	mustSucceed(t, err)

	categoryID, err := domain.ParseCategoryID(vpnCategoryID)
	mustSucceed(t, err)

	ticket, err := domain.NewTicket(
		domain.NewTicketParams{
			ID:          ticketID,
			RequesterID: requesterID,
			CategoryID:  categoryID,
			Title:       "VPN does not work after update",
			Description: "The VPN client cannot establish a connection.",
			Priority:    domain.PriorityNormal,
			SLA: domain.SLATarget{
				FirstResponse: 2 * time.Hour,
				Resolution:    24 * time.Hour,
			},
		},
		time.Date(
			2026,
			time.July,
			28,
			10,
			0,
			0,
			0,
			time.UTC,
		),
	)
	mustSucceed(t, err)

	return ticket
}

func mustNewUserID(t *testing.T) domain.UserID {
	t.Helper()

	id, err := domain.NewUserID()
	mustSucceed(t, err)

	return id
}

func mustSucceed(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
