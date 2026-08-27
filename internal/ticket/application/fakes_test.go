package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type fakeTicketRepository struct {
	tickets   map[domain.TicketID]domain.TicketSnapshot
	createErr error
	updateErr error
}

func newFakeTicketRepository() *fakeTicketRepository {
	return &fakeTicketRepository{
		tickets: make(
			map[domain.TicketID]domain.TicketSnapshot,
		),
	}
}

func (repository *fakeTicketRepository) Create(
	_ context.Context,
	ticket *domain.Ticket,
) error {
	if repository.createErr != nil {
		return repository.createErr
	}

	snapshot := ticket.Snapshot()
	if _, exists := repository.tickets[snapshot.ID]; exists {
		return fmt.Errorf(
			"%w: %s",
			ports.ErrTicketAlreadyExists,
			snapshot.ID,
		)
	}

	repository.tickets[snapshot.ID] = snapshot
	return nil
}

func (repository *fakeTicketRepository) GetByID(
	_ context.Context,
	ticketID domain.TicketID,
) (*domain.Ticket, error) {
	snapshot, exists := repository.tickets[ticketID]
	if !exists {
		return nil, fmt.Errorf(
			"%w: %s",
			ports.ErrTicketNotFound,
			ticketID,
		)
	}

	ticket, err := domain.RestoreTicket(snapshot)
	if err != nil {
		return nil, fmt.Errorf("restore fake ticket: %w", err)
	}

	return ticket, nil
}

func (repository *fakeTicketRepository) Update(
	_ context.Context,
	ticket *domain.Ticket,
	expectedVersion uint64,
) error {
	if repository.updateErr != nil {
		return repository.updateErr
	}

	snapshot := ticket.Snapshot()
	stored, exists := repository.tickets[snapshot.ID]
	if !exists {
		return fmt.Errorf(
			"%w: %s",
			ports.ErrTicketNotFound,
			snapshot.ID,
		)
	}

	if stored.Version != expectedVersion {
		return ports.VersionConflictError{
			TicketID: snapshot.ID,
			Expected: expectedVersion,
			Actual:   stored.Version,
		}
	}

	if snapshot.Version <= expectedVersion {
		return fmt.Errorf(
			"new version %d must exceed %d",
			snapshot.Version,
			expectedVersion,
		)
	}

	repository.tickets[snapshot.ID] = snapshot
	return nil
}

func (repository *fakeTicketRepository) put(ticket *domain.Ticket) {
	snapshot := ticket.Snapshot()
	repository.tickets[snapshot.ID] = snapshot
}

type fakeSLAPolicyRepository struct {
	targets map[domain.Priority]domain.SLATarget
	err     error
	calls   int
}

func (repository *fakeSLAPolicyRepository) GetByPriority(
	_ context.Context,
	priority domain.Priority,
) (domain.SLATarget, error) {
	repository.calls++

	if repository.err != nil {
		return domain.SLATarget{}, repository.err
	}

	target, exists := repository.targets[priority]
	if !exists {
		return domain.SLATarget{}, fmt.Errorf(
			"%w: %s",
			ports.ErrSLAPolicyNotFound,
			priority,
		)
	}

	return target, nil
}

type fakeCategoryRepository struct {
	active map[domain.CategoryID]bool
	err    error
	calls  int
}

func (repository *fakeCategoryRepository) IsActive(
	_ context.Context,
	categoryID domain.CategoryID,
) (bool, error) {
	repository.calls++

	if repository.err != nil {
		return false, repository.err
	}

	return repository.active[categoryID], nil
}

type fakeUnitOfWork struct {
	repositories ports.Repositories
	err          error
	calls        int
}

func (unit *fakeUnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories ports.Repositories) error,
) error {
	unit.calls++

	if unit.err != nil {
		return unit.err
	}

	return fn(unit.repositories)
}

type fixedClock struct {
	value time.Time
}

func (clock fixedClock) Now() time.Time {
	return clock.value
}

type fixedTicketIDGenerator struct {
	id  domain.TicketID
	err error
}

func (generator fixedTicketIDGenerator) NewTicketID() (
	domain.TicketID,
	error,
) {
	return generator.id, generator.err
}

type testEnvironment struct {
	tickets    *fakeTicketRepository
	sla        *fakeSLAPolicyRepository
	categories *fakeCategoryRepository
	unit       *fakeUnitOfWork
}

func newTestEnvironment() testEnvironment {
	tickets := newFakeTicketRepository()
	sla := &fakeSLAPolicyRepository{
		targets: map[domain.Priority]domain.SLATarget{
			domain.PriorityNormal: {
				FirstResponse: 2 * time.Hour,
				Resolution:    24 * time.Hour,
			},
			domain.PriorityHigh: {
				FirstResponse: 30 * time.Minute,
				Resolution:    8 * time.Hour,
			},
		},
	}
	categories := &fakeCategoryRepository{
		active: make(map[domain.CategoryID]bool),
	}
	unit := &fakeUnitOfWork{
		repositories: ports.Repositories{
			Tickets:     tickets,
			SLAPolicies: sla,
			Categories:  categories,
		},
	}

	return testEnvironment{
		tickets:    tickets,
		sla:        sla,
		categories: categories,
		unit:       unit,
	}
}

const (
	requesterIDText = "019d0000-0000-7000-8000-000000000101"
	otherUserIDText = "019d0000-0000-7000-8000-000000000102"
	operatorIDText  = "019d0000-0000-7000-8000-000000000201"
	managerIDText   = "019d0000-0000-7000-8000-000000000202"
	ticketIDText    = "019d0000-0000-7000-8000-000000000301"
	categoryIDText  = "019d0000-0000-7000-8000-000000000001"
)

var baseTime = time.Date(
	2026,
	time.August,
	10,
	9,
	0,
	0,
	0,
	time.UTC,
)

func mustUserID(t *testing.T, raw string) domain.UserID {
	t.Helper()

	id, err := domain.ParseUserID(raw)
	if err != nil {
		t.Fatalf("ParseUserID() returned error: %v", err)
	}

	return id
}

func mustTicketID(t *testing.T) domain.TicketID {
	t.Helper()

	id, err := domain.ParseTicketID(ticketIDText)
	if err != nil {
		t.Fatalf("ParseTicketID() returned error: %v", err)
	}

	return id
}

func mustCategoryID(t *testing.T) domain.CategoryID {
	t.Helper()

	id, err := domain.ParseCategoryID(categoryIDText)
	if err != nil {
		t.Fatalf("ParseCategoryID() returned error: %v", err)
	}

	return id
}

func newStoredTicket(
	t *testing.T,
	requesterID domain.UserID,
) *domain.Ticket {
	t.Helper()

	ticket, err := domain.NewTicket(
		domain.NewTicketParams{
			ID:          mustTicketID(t),
			RequesterID: requesterID,
			CategoryID:  mustCategoryID(t),
			Title:       "VPN connection is unavailable",
			Description: "The client cannot connect after the system update.",
			Priority:    domain.PriorityNormal,
			SLA: domain.SLATarget{
				FirstResponse: 2 * time.Hour,
				Resolution:    24 * time.Hour,
			},
		},
		baseTime,
	)
	if err != nil {
		t.Fatalf("NewTicket() returned error: %v", err)
	}

	return ticket
}

func requireErrorIs(
	t *testing.T,
	err error,
	target error,
) {
	t.Helper()

	if !errors.Is(err, target) {
		t.Fatalf("expected %v, got %v", target, err)
	}
}
