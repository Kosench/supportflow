package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application"
	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func TestCreateTicketUsesActorAndCurrentSLA(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	categoryID := mustCategoryID(t)
	environment.categories.active[categoryID] = true

	handler, err := application.NewCreateTicketHandler(
		environment.unit,
		fixedClock{value: baseTime},
		fixedTicketIDGenerator{id: mustTicketID(t)},
	)
	if err != nil {
		t.Fatalf("NewCreateTicketHandler() returned error: %v", err)
	}

	view, err := handler.Handle(
		context.Background(),
		application.CreateTicketCommand{
			Actor: application.Actor{
				UserID: requesterID,
				Role:   application.RoleUser,
			},
			CategoryID:  categoryIDText,
			Title:       "VPN connection is unavailable",
			Description: "The client cannot connect after the system update.",
			Priority:    "normal",
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if view.ID != ticketIDText {
		t.Errorf("unexpected ticket ID: %s", view.ID)
	}

	if view.RequesterID != requesterIDText {
		t.Errorf("unexpected requester ID: %s", view.RequesterID)
	}

	if view.Priority != string(domain.PriorityNormal) {
		t.Errorf("unexpected priority: %s", view.Priority)
	}

	if view.Version != 1 {
		t.Errorf("unexpected version: %d", view.Version)
	}

	if !view.FirstResponseDeadlineAt.Equal(
		baseTime.Add(2 * time.Hour),
	) {
		t.Errorf(
			"unexpected first response deadline: %s",
			view.FirstResponseDeadlineAt,
		)
	}

	if !view.ResolutionDeadlineAt.Equal(
		baseTime.Add(24 * time.Hour),
	) {
		t.Errorf(
			"unexpected resolution deadline: %s",
			view.ResolutionDeadlineAt,
		)
	}

	if environment.categories.calls != 1 {
		t.Errorf(
			"unexpected category calls: %d",
			environment.categories.calls,
		)
	}

	if environment.sla.calls != 1 {
		t.Errorf("unexpected SLA calls: %d", environment.sla.calls)
	}
}

func TestCreateTicketRejectsUnavailableCategory(t *testing.T) {
	environment := newTestEnvironment()

	handler, err := application.NewCreateTicketHandler(
		environment.unit,
		fixedClock{value: baseTime},
		fixedTicketIDGenerator{id: mustTicketID(t)},
	)
	if err != nil {
		t.Fatalf("NewCreateTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.CreateTicketCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, requesterIDText),
				Role:   application.RoleUser,
			},
			CategoryID:  categoryIDText,
			Title:       "VPN connection is unavailable",
			Description: "The client cannot connect after the system update.",
			Priority:    "NORMAL",
		},
	)
	requireErrorIs(t, err, application.ErrCategoryUnavailable)

	if environment.sla.calls != 0 {
		t.Error("SLA repository must not be called")
	}

	if len(environment.tickets.tickets) != 0 {
		t.Error("ticket must not be created")
	}
}

func TestCreateTicketRejectsOperator(t *testing.T) {
	environment := newTestEnvironment()

	handler, err := application.NewCreateTicketHandler(
		environment.unit,
		fixedClock{value: baseTime},
		fixedTicketIDGenerator{id: mustTicketID(t)},
	)
	if err != nil {
		t.Fatalf("NewCreateTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.CreateTicketCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, operatorIDText),
				Role:   application.RoleOperator,
			},
			CategoryID: categoryIDText,
		},
	)
	requireErrorIs(t, err, application.ErrPermissionDenied)

	if environment.unit.calls != 0 {
		t.Error("transaction must not start for forbidden command")
	}
}

func TestGetTicketEnforcesObjectLevelAuthorization(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))

	handler, err := application.NewGetTicketHandler(
		environment.tickets,
	)
	if err != nil {
		t.Fatalf("NewGetTicketHandler() returned error: %v", err)
	}

	tests := []struct {
		name    string
		actor   application.Actor
		wantErr error
	}{
		{
			name: "requester can read",
			actor: application.Actor{
				UserID: requesterID,
				Role:   application.RoleUser,
			},
		},
		{
			name: "other user is denied",
			actor: application.Actor{
				UserID: mustUserID(t, otherUserIDText),
				Role:   application.RoleUser,
			},
			wantErr: application.ErrPermissionDenied,
		},
		{
			name: "manager can read",
			actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleManager,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, err := handler.Handle(
				context.Background(),
				application.GetTicketQuery{
					Actor:    test.actor,
					TicketID: ticketIDText,
				},
			)

			if test.wantErr != nil {
				requireErrorIs(t, err, test.wantErr)
				return
			}

			if err != nil {
				t.Fatalf("Handle() returned error: %v", err)
			}

			if view.ID != ticketIDText {
				t.Errorf("unexpected ticket ID: %s", view.ID)
			}
		})
	}
}

func TestChangePriorityByAssignee(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	if err := ticket.Assign(
		operatorID,
		baseTime.Add(time.Minute),
	); err != nil {
		t.Fatalf("Assign() returned error: %v", err)
	}
	environment.tickets.put(ticket)

	handler, err := application.NewChangePriorityHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewChangePriorityHandler() returned error: %v", err)
	}

	view, err := handler.Handle(
		context.Background(),
		application.ChangePriorityCommand{
			Actor: application.Actor{
				UserID: operatorID,
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			Priority:        "HIGH",
			Reason:          "Production access is blocked",
			ExpectedVersion: 2,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if view.Priority != string(domain.PriorityHigh) {
		t.Errorf("unexpected priority: %s", view.Priority)
	}

	if view.Version != 3 {
		t.Errorf("unexpected version: %d", view.Version)
	}

	if !view.ResolutionDeadlineAt.Equal(
		baseTime.Add(8 * time.Hour),
	) {
		t.Errorf(
			"unexpected resolution deadline: %s",
			view.ResolutionDeadlineAt,
		)
	}
}

func TestChangePriorityRejectsStaleClientVersion(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	if err := ticket.Assign(
		operatorID,
		baseTime.Add(time.Minute),
	); err != nil {
		t.Fatalf("Assign() returned error: %v", err)
	}
	environment.tickets.put(ticket)

	handler, err := application.NewChangePriorityHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewChangePriorityHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.ChangePriorityCommand{
			Actor: application.Actor{
				UserID: operatorID,
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			Priority:        "HIGH",
			Reason:          "Production access is blocked",
			ExpectedVersion: 1,
		},
	)
	requireErrorIs(t, err, ports.ErrVersionConflict)

	if environment.sla.calls != 0 {
		t.Error("SLA repository must not be called after version conflict")
	}

	stored, getErr := environment.tickets.GetByID(
		context.Background(),
		mustTicketID(t),
	)
	if getErr != nil {
		t.Fatalf("GetByID() returned error: %v", getErr)
	}

	if stored.Snapshot().Priority != domain.PriorityNormal {
		t.Error("stale command changed stored priority")
	}
}

func TestChangePriorityRejectsOtherOperator(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	assigneeID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	if err := ticket.Assign(
		assigneeID,
		baseTime.Add(time.Minute),
	); err != nil {
		t.Fatalf("Assign() returned error: %v", err)
	}
	environment.tickets.put(ticket)

	handler, err := application.NewChangePriorityHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewChangePriorityHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.ChangePriorityCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			Priority:        "HIGH",
			Reason:          "Production access is blocked",
			ExpectedVersion: 1,
		},
	)
	requireErrorIs(t, err, application.ErrPermissionDenied)

	if environment.sla.calls != 0 {
		t.Error("SLA repository must not be called after denied access")
	}
}

func TestCancelTicketByRequester(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))

	handler, err := application.NewCancelTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewCancelTicketHandler() returned error: %v", err)
	}

	view, err := handler.Handle(
		context.Background(),
		application.CancelTicketCommand{
			Actor: application.Actor{
				UserID: requesterID,
				Role:   application.RoleUser,
			},
			TicketID:        ticketIDText,
			Reason:          "The issue is no longer relevant",
			ExpectedVersion: 1,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if view.Status != string(domain.StatusCancelled) {
		t.Errorf("unexpected status: %s", view.Status)
	}

	if view.Version != 2 {
		t.Errorf("unexpected version: %d", view.Version)
	}
}

func TestCancelTicketStillUsesDomainTransitionRules(t *testing.T) {
	environment := newTestEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	if err := ticket.Assign(
		operatorID,
		baseTime.Add(time.Minute),
	); err != nil {
		t.Fatalf("Assign() returned error: %v", err)
	}

	if err := ticket.StartProgress(
		baseTime.Add(2 * time.Minute),
	); err != nil {
		t.Fatalf("StartProgress() returned error: %v", err)
	}
	environment.tickets.put(ticket)

	handler, err := application.NewCancelTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(3 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewCancelTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.CancelTicketCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleManager,
			},
			TicketID:        ticketIDText,
			Reason:          "Manager requested cancellation",
			ExpectedVersion: 3,
		},
	)
	requireErrorIs(t, err, domain.ErrInvalidTransition)

	stored, getErr := environment.tickets.GetByID(
		context.Background(),
		mustTicketID(t),
	)
	if getErr != nil {
		t.Fatalf("GetByID() returned error: %v", getErr)
	}

	if stored.Snapshot().Status != domain.StatusInProgress {
		t.Error("failed cancellation changed stored status")
	}
}

func TestHandlersRejectUnauthenticatedActor(t *testing.T) {
	environment := newTestEnvironment()
	handler, err := application.NewGetTicketHandler(
		environment.tickets,
	)
	if err != nil {
		t.Fatalf("NewGetTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.GetTicketQuery{
			Actor:    application.Actor{},
			TicketID: ticketIDText,
		},
	)
	requireErrorIs(t, err, application.ErrUnauthenticated)
}

func TestConstructorRejectsMissingDependency(t *testing.T) {
	_, err := application.NewCancelTicketHandler(
		nil,
		fixedClock{value: baseTime},
	)

	if !errors.Is(err, application.ErrInvalidDependency) {
		t.Fatalf("expected ErrInvalidDependency, got %v", err)
	}
}
