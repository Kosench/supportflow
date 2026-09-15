package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type workflowClock struct {
	value time.Time
}

func (clock *workflowClock) Now() time.Time {
	return clock.value
}

func (clock *workflowClock) Set(value time.Time) {
	clock.value = value
}

func TestWorkflowLifecycleHappyPath(t *testing.T) {
	environment := newAssignmentEnvironment()
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

	clock := &workflowClock{value: baseTime.Add(2 * time.Minute)}
	handler, err := application.NewWorkflowHandler(
		environment.unit,
		clock,
	)
	if err != nil {
		t.Fatalf("NewWorkflowHandler() returned error: %v", err)
	}

	operator := application.Actor{
		UserID: operatorID,
		Role:   application.RoleOperator,
	}
	requester := application.Actor{
		UserID: requesterID,
		Role:   application.RoleUser,
	}

	result, err := handler.StartProgress(
		context.Background(),
		application.WorkflowCommand{
			Actor:           operator,
			TicketID:        ticketIDText,
			ExpectedVersion: 2,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusInProgress,
		3,
	)

	clock.Set(baseTime.Add(3 * time.Minute))
	result, err = handler.WaitForCustomer(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor:           operator,
			TicketID:        ticketIDText,
			Reason:          "Please attach the VPN client log",
			ExpectedVersion: 3,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusWaitingCustomer,
		4,
	)

	if result.Ticket.WaitingReason == "" {
		t.Error("waiting reason must be stored")
	}

	clock.Set(baseTime.Add(4 * time.Minute))
	result, err = handler.RecordFirstResponse(
		context.Background(),
		application.WorkflowCommand{
			Actor:           operator,
			TicketID:        ticketIDText,
			ExpectedVersion: 4,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusWaitingCustomer,
		5,
	)

	if result.Ticket.FirstRespondedAt == nil ||
		!result.Ticket.FirstRespondedAt.Equal(clock.Now()) {
		t.Fatalf(
			"unexpected first response time: %v",
			result.Ticket.FirstRespondedAt,
		)
	}
	firstResponseTime := *result.Ticket.FirstRespondedAt

	clock.Set(baseTime.Add(5 * time.Minute))
	result, err = handler.ResumeProgress(
		context.Background(),
		application.WorkflowCommand{
			Actor:           requester,
			TicketID:        ticketIDText,
			ExpectedVersion: 5,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusInProgress,
		6,
	)

	if result.Ticket.WaitingReason != "" {
		t.Error("waiting reason must be cleared after resume")
	}

	clock.Set(baseTime.Add(6 * time.Minute))
	result, err = handler.Resolve(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor:           operator,
			TicketID:        ticketIDText,
			Reason:          "The expired VPN certificate was renewed",
			ExpectedVersion: 6,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusResolved,
		7,
	)

	if result.Ticket.ResolvedAt == nil ||
		result.Ticket.Resolution == "" {
		t.Error("resolved ticket must contain resolution and time")
	}

	clock.Set(baseTime.Add(7 * time.Minute))
	result, err = handler.Close(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor:           requester,
			TicketID:        ticketIDText,
			Reason:          "The connection works again",
			ExpectedVersion: 7,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusClosed,
		8,
	)

	if result.Ticket.ClosedAt == nil {
		t.Error("closed ticket must contain closed time")
	}

	clock.Set(baseTime.Add(8 * time.Minute))
	result, err = handler.Reopen(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor:           requester,
			TicketID:        ticketIDText,
			Reason:          "The problem returned after reconnect",
			ExpectedVersion: 8,
		},
	)
	assertWorkflowResult(
		t,
		result,
		err,
		domain.StatusReopened,
		9,
	)

	if result.Ticket.Resolution != "" ||
		result.Ticket.ResolvedAt != nil ||
		result.Ticket.ClosedAt != nil {
		t.Error("reopened ticket must clear completion fields")
	}

	if result.Ticket.FirstRespondedAt == nil ||
		!result.Ticket.FirstRespondedAt.Equal(firstResponseTime) {
		t.Error("reopen must preserve first response time")
	}

	wantDeadline := clock.Now().Add(24 * time.Hour)
	if !result.Ticket.ResolutionDeadlineAt.Equal(wantDeadline) {
		t.Errorf(
			"unexpected resolution deadline: got %s want %s",
			result.Ticket.ResolutionDeadlineAt,
			wantDeadline,
		)
	}

	if environment.sla.calls != 1 {
		t.Errorf("unexpected SLA calls: %d", environment.sla.calls)
	}
}

func TestRecordFirstResponseIsIdempotent(t *testing.T) {
	environment := newAssignmentEnvironment()
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

	clock := &workflowClock{value: baseTime.Add(2 * time.Minute)}
	handler, err := application.NewWorkflowHandler(
		environment.unit,
		clock,
	)
	if err != nil {
		t.Fatalf("NewWorkflowHandler() returned error: %v", err)
	}

	command := application.WorkflowCommand{
		Actor: application.Actor{
			UserID: operatorID,
			Role:   application.RoleOperator,
		},
		TicketID:        ticketIDText,
		ExpectedVersion: 2,
	}

	first, err := handler.RecordFirstResponse(
		context.Background(),
		command,
	)
	assertWorkflowResult(
		t,
		first,
		err,
		domain.StatusAssigned,
		3,
	)

	clock.Set(baseTime.Add(3 * time.Minute))
	command.ExpectedVersion = first.Ticket.Version
	second, err := handler.RecordFirstResponse(
		context.Background(),
		command,
	)
	if err != nil {
		t.Fatalf("second RecordFirstResponse() returned error: %v", err)
	}

	if second.Changed {
		t.Error("second first-response call must not change ticket")
	}

	if second.Ticket.Version != first.Ticket.Version {
		t.Errorf(
			"idempotent call changed version: got %d want %d",
			second.Ticket.Version,
			first.Ticket.Version,
		)
	}

	if second.Ticket.FirstRespondedAt == nil ||
		first.Ticket.FirstRespondedAt == nil ||
		!second.Ticket.FirstRespondedAt.Equal(
			*first.Ticket.FirstRespondedAt,
		) {
		t.Error("idempotent call changed first response time")
	}
}

func TestWorkflowAuthorizationPrecedesVersionCheck(t *testing.T) {
	environment := newAssignmentEnvironment()
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

	handler, err := application.NewWorkflowHandler(
		environment.unit,
		&workflowClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewWorkflowHandler() returned error: %v", err)
	}

	_, err = handler.StartProgress(
		context.Background(),
		application.WorkflowCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			ExpectedVersion: 99,
		},
	)
	requireErrorIs(t, err, application.ErrPermissionDenied)
}

func TestManagerCannotBypassDomainTransition(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))

	handler, err := application.NewWorkflowHandler(
		environment.unit,
		&workflowClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewWorkflowHandler() returned error: %v", err)
	}

	_, err = handler.Close(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleManager,
			},
			TicketID:        ticketIDText,
			Reason:          "Manager requested closure",
			ExpectedVersion: 1,
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

	if stored.Snapshot().Status != domain.StatusNew {
		t.Error("failed transition changed stored ticket")
	}
}

func TestUnassignIsRestrictedToManager(t *testing.T) {
	tests := []struct {
		name      string
		actor     application.Actor
		wantError error
	}{
		{
			name: "manager",
			actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleManager,
			},
		},
		{
			name: "requester",
			actor: application.Actor{
				UserID: mustUserID(t, requesterIDText),
				Role:   application.RoleUser,
			},
			wantError: application.ErrPermissionDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := newAssignmentEnvironment()
			requesterID := mustUserID(t, requesterIDText)
			ticket := newStoredTicket(t, requesterID)

			if err := ticket.Assign(
				mustUserID(t, operatorIDText),
				baseTime.Add(time.Minute),
			); err != nil {
				t.Fatalf("Assign() returned error: %v", err)
			}
			environment.tickets.put(ticket)

			handler, err := application.NewWorkflowHandler(
				environment.unit,
				&workflowClock{
					value: baseTime.Add(2 * time.Minute),
				},
			)
			if err != nil {
				t.Fatalf(
					"NewWorkflowHandler() returned error: %v",
					err,
				)
			}

			result, err := handler.Unassign(
				context.Background(),
				application.WorkflowCommand{
					Actor:           test.actor,
					TicketID:        ticketIDText,
					ExpectedVersion: 2,
				},
			)
			if test.wantError != nil {
				requireErrorIs(t, err, test.wantError)
				return
			}

			assertWorkflowResult(
				t,
				result,
				err,
				domain.StatusNew,
				3,
			)

			if result.Ticket.AssigneeID != nil {
				t.Error("unassigned ticket still has assignee")
			}
		})
	}
}

func TestDeniedReopenDoesNotLoadSLA(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	steps := []func() error{
		func() error {
			return ticket.Assign(
				operatorID,
				baseTime.Add(time.Minute),
			)
		},
		func() error {
			return ticket.StartProgress(
				baseTime.Add(2 * time.Minute),
			)
		},
		func() error {
			return ticket.Resolve(
				"Certificate renewed",
				baseTime.Add(3*time.Minute),
			)
		},
		func() error {
			return ticket.Close(
				"Requester confirmed the fix",
				baseTime.Add(4*time.Minute),
			)
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("prepare closed ticket: %v", err)
		}
	}
	environment.tickets.put(ticket)

	handler, err := application.NewWorkflowHandler(
		environment.unit,
		&workflowClock{value: baseTime.Add(5 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewWorkflowHandler() returned error: %v", err)
	}

	_, err = handler.Reopen(
		context.Background(),
		application.ReasonedWorkflowCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, otherUserIDText),
				Role:   application.RoleUser,
			},
			TicketID:        ticketIDText,
			Reason:          "Trying to reopen another ticket",
			ExpectedVersion: ticket.Snapshot().Version,
		},
	)
	requireErrorIs(t, err, application.ErrPermissionDenied)

	if environment.sla.calls != 0 {
		t.Error("denied reopen must not load SLA")
	}
}

func assertWorkflowResult(
	t *testing.T,
	result application.WorkflowResult,
	err error,
	wantStatus domain.Status,
	wantVersion uint64,
) {
	t.Helper()

	if err != nil {
		t.Fatalf("workflow returned error: %v", err)
	}

	if !result.Changed {
		t.Error("workflow must report a change")
	}

	if result.Ticket.Status != string(wantStatus) {
		t.Errorf(
			"unexpected status: got %s want %s",
			result.Ticket.Status,
			wantStatus,
		)
	}

	if result.Ticket.Version != wantVersion {
		t.Errorf(
			"unexpected version: got %d want %d",
			result.Ticket.Version,
			wantVersion,
		)
	}
}
