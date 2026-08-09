package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTicketCreatesValidAggregate(t *testing.T) {
	now := testTime()
	params := validTicketParams(t)

	ticket, err := NewTicket(params, now)
	if err != nil {
		t.Fatalf("NewTicket() returned error: %v", err)
	}

	snapshot := ticket.Snapshot()

	if snapshot.Status != StatusNew {
		t.Errorf("unexpected status: %s", snapshot.Status)
	}

	if snapshot.Version != 1 {
		t.Errorf("unexpected version: %d", snapshot.Version)
	}

	if snapshot.AssigneeID != nil {
		t.Error("new ticket must not have assignee")
	}

	wantFirstResponseDeadline := now.Add(2 * time.Hour)
	if !snapshot.FirstResponseDeadlineAt.Equal(wantFirstResponseDeadline) {
		t.Errorf(
			"unexpected first response deadline: %s",
			snapshot.FirstResponseDeadlineAt,
		)
	}

	wantResolutionDeadline := now.Add(24 * time.Hour)
	if !snapshot.ResolutionDeadlineAt.Equal(wantResolutionDeadline) {
		t.Errorf(
			"unexpected resolution deadline: %s",
			snapshot.ResolutionDeadlineAt,
		)
	}

	events := ticket.Events()
	if len(events) != 1 {
		t.Fatalf("unexpected event count: %d", len(events))
	}

	if events[0].Type != EventTicketCreated {
		t.Errorf("unexpected event type: %s", events[0].Type)
	}

	if events[0].AggregateVersion != 1 {
		t.Errorf(
			"unexpected aggregate version: %d",
			events[0].AggregateVersion,
		)
	}
}

func TestNewTicketRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NewTicketParams)
	}{
		{
			name: "zero ticket ID",
			mutate: func(params *NewTicketParams) {
				params.ID = TicketID{}
			},
		},
		{
			name: "short title",
			mutate: func(params *NewTicketParams) {
				params.Title = "VPN"
			},
		},
		{
			name: "short description",
			mutate: func(params *NewTicketParams) {
				params.Description = "broken"
			},
		},
		{
			name: "unknown priority",
			mutate: func(params *NewTicketParams) {
				params.Priority = Priority("URGENT")
			},
		},
		{
			name: "invalid SLA",
			mutate: func(params *NewTicketParams) {
				params.SLA.Resolution = 0
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := validTicketParams(t)
			test.mutate(&params)

			_, err := NewTicket(params, testTime())
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestTicketLifecycle(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()
	assigneeID := mustNewUserID(t)

	mustSucceed(t, ticket.Assign(assigneeID, now.Add(time.Minute)))
	assertStatus(t, ticket, StatusAssigned)

	mustSucceed(t, ticket.StartProgress(now.Add(2*time.Minute)))
	assertStatus(t, ticket, StatusInProgress)

	mustSucceed(t, ticket.WaitForCustomer(
		"Need VPN client logs",
		now.Add(3*time.Minute),
	))
	assertStatus(t, ticket, StatusWaitingCustomer)

	mustSucceed(t, ticket.ResumeProgress(now.Add(4*time.Minute)))
	assertStatus(t, ticket, StatusInProgress)

	mustSucceed(t, ticket.Resolve(
		"Reinstalled the VPN profile",
		now.Add(5*time.Minute),
	))
	assertStatus(t, ticket, StatusResolved)

	mustSucceed(t, ticket.Close(
		"Requester confirmed the solution",
		now.Add(6*time.Minute),
	))
	assertStatus(t, ticket, StatusClosed)

	mustSucceed(t, ticket.Reopen(
		"VPN stopped working again",
		8*time.Hour,
		now.Add(7*time.Minute),
	))
	assertStatus(t, ticket, StatusReopened)

	mustSucceed(t, ticket.StartProgress(now.Add(8*time.Minute)))
	assertStatus(t, ticket, StatusInProgress)

	snapshot := ticket.Snapshot()
	if snapshot.Version != 9 {
		t.Errorf("unexpected version: got %d want 9", snapshot.Version)
	}

	if snapshot.ResolvedAt != nil {
		t.Error("reopened ticket must not have resolved_at")
	}

	if snapshot.ClosedAt != nil {
		t.Error("reopened ticket must not have closed_at")
	}

	wantDeadline := now.Add(7*time.Minute + 8*time.Hour)
	if !snapshot.ResolutionDeadlineAt.Equal(wantDeadline) {
		t.Errorf(
			"unexpected reopened deadline: got %s want %s",
			snapshot.ResolutionDeadlineAt,
			wantDeadline,
		)
	}
}

func TestTicketRejectsInvalidTransition(t *testing.T) {
	ticket := newTestTicket(t)

	err := ticket.Resolve(
		"Cannot resolve a new ticket",
		testTime().Add(time.Minute),
	)

	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	var transitionErr TransitionError
	if !errors.As(err, &transitionErr) {
		t.Fatalf("expected TransitionError, got %T", err)
	}

	if transitionErr.From != StatusNew {
		t.Errorf("unexpected source status: %s", transitionErr.From)
	}

	if transitionErr.To != StatusResolved {
		t.Errorf("unexpected target status: %s", transitionErr.To)
	}

	if ticket.Snapshot().Version != 1 {
		t.Error("failed operation must not change version")
	}
}

func TestTicketUsesExplicitProgressOperations(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(time.Minute),
	))

	err := ticket.ResumeProgress(now.Add(2 * time.Minute))
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	mustSucceed(t, ticket.StartProgress(now.Add(2*time.Minute)))
	mustSucceed(t, ticket.WaitForCustomer(
		"Need additional diagnostic data",
		now.Add(3*time.Minute),
	))

	err = ticket.StartProgress(now.Add(4 * time.Minute))
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	mustSucceed(t, ticket.ResumeProgress(now.Add(4*time.Minute)))
	assertStatus(t, ticket, StatusInProgress)
}

func TestAssignCanReassignActiveTicket(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()
	firstAssigneeID := mustNewUserID(t)
	secondAssigneeID := mustNewUserID(t)

	mustSucceed(t, ticket.Assign(
		firstAssigneeID,
		now.Add(time.Minute),
	))
	mustSucceed(t, ticket.StartProgress(now.Add(2*time.Minute)))
	mustSucceed(t, ticket.Assign(
		secondAssigneeID,
		now.Add(3*time.Minute),
	))

	snapshot := ticket.Snapshot()

	if snapshot.Status != StatusInProgress {
		t.Errorf("reassignment changed status: %s", snapshot.Status)
	}

	if snapshot.AssigneeID == nil {
		t.Fatal("ticket has no assignee")
	}

	if *snapshot.AssigneeID != secondAssigneeID {
		t.Errorf(
			"unexpected assignee: got %s want %s",
			snapshot.AssigneeID.String(),
			secondAssigneeID.String(),
		)
	}

	err := ticket.Assign(
		secondAssigneeID,
		now.Add(4*time.Minute),
	)
	if !errors.Is(err, ErrNoChanges) {
		t.Errorf("expected ErrNoChanges, got %v", err)
	}
}

func TestCancelClearsAssignee(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(time.Minute),
	))
	mustSucceed(t, ticket.Cancel(
		"Request is no longer needed",
		now.Add(2*time.Minute),
	))

	snapshot := ticket.Snapshot()
	if snapshot.Status != StatusCancelled {
		t.Errorf("unexpected status: %s", snapshot.Status)
	}

	if snapshot.AssigneeID != nil {
		t.Error("cancelled ticket must not have assignee")
	}
}

func TestMarkFirstRespondedIsIdempotent(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(time.Minute),
	))

	recorded, err := ticket.MarkFirstResponded(now.Add(2 * time.Minute))
	mustSucceed(t, err)
	if !recorded {
		t.Fatal("first call must record response")
	}

	versionAfterFirstCall := ticket.Snapshot().Version
	firstRespondedAt := ticket.Snapshot().FirstRespondedAt

	recorded, err = ticket.MarkFirstResponded(now.Add(3 * time.Minute))
	mustSucceed(t, err)
	if recorded {
		t.Fatal("second call must not record response")
	}

	snapshot := ticket.Snapshot()
	if snapshot.Version != versionAfterFirstCall {
		t.Error("idempotent call changed version")
	}

	if !snapshot.FirstRespondedAt.Equal(*firstRespondedAt) {
		t.Error("idempotent call changed first response time")
	}
}

func TestChangePriorityRecalculatesOnlyOpenDeadlines(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(time.Minute),
	))

	recorded, err := ticket.MarkFirstResponded(now.Add(10 * time.Minute))
	mustSucceed(t, err)
	if !recorded {
		t.Fatal("first response was not recorded")
	}

	firstDeadlineBefore := ticket.Snapshot().FirstResponseDeadlineAt

	mustSucceed(t, ticket.ChangePriority(
		PriorityHigh,
		"Incident affects the entire department",
		SLATarget{
			FirstResponse: 30 * time.Minute,
			Resolution:    8 * time.Hour,
		},
		now.Add(20*time.Minute),
	))

	snapshot := ticket.Snapshot()
	if snapshot.Priority != PriorityHigh {
		t.Errorf("unexpected priority: %s", snapshot.Priority)
	}

	if !snapshot.FirstResponseDeadlineAt.Equal(firstDeadlineBefore) {
		t.Error("completed first response deadline was changed")
	}

	wantResolutionDeadline := now.Add(8 * time.Hour)
	if !snapshot.ResolutionDeadlineAt.Equal(wantResolutionDeadline) {
		t.Errorf(
			"unexpected resolution deadline: got %s want %s",
			snapshot.ResolutionDeadlineAt,
			wantResolutionDeadline,
		)
	}
}

func TestChangePriorityAfterReopenUsesNewResolutionCycle(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(time.Minute),
	))
	mustSucceed(t, ticket.StartProgress(now.Add(2*time.Minute)))
	mustSucceed(t, ticket.Resolve(
		"Recreated the VPN configuration",
		now.Add(3*time.Minute),
	))
	mustSucceed(t, ticket.Close(
		"Requester confirmed the solution",
		now.Add(4*time.Minute),
	))

	reopenedAt := now.Add(5 * time.Minute)
	mustSucceed(t, ticket.Reopen(
		"The problem appeared again",
		24*time.Hour,
		reopenedAt,
	))

	mustSucceed(t, ticket.ChangePriority(
		PriorityHigh,
		"Repeated failure affects all remote work",
		SLATarget{
			FirstResponse: 30 * time.Minute,
			Resolution:    8 * time.Hour,
		},
		now.Add(6*time.Minute),
	))

	snapshot := ticket.Snapshot()
	wantDeadline := reopenedAt.Add(8 * time.Hour)
	if !snapshot.ResolutionDeadlineAt.Equal(wantDeadline) {
		t.Errorf(
			"unexpected resolution deadline: got %s want %s",
			snapshot.ResolutionDeadlineAt,
			wantDeadline,
		)
	}
}

func TestMutationRejectsTimeBeforeLastUpdate(t *testing.T) {
	ticket := newTestTicket(t)
	now := testTime()

	mustSucceed(t, ticket.Assign(
		mustNewUserID(t),
		now.Add(2*time.Minute),
	))

	err := ticket.StartProgress(now.Add(time.Minute))
	if !errors.Is(err, ErrInvalidTime) {
		t.Fatalf("expected ErrInvalidTime, got %v", err)
	}

	if ticket.Snapshot().Status != StatusAssigned {
		t.Error("failed operation changed status")
	}
}

func TestEventsReturnsSliceCopy(t *testing.T) {
	ticket := newTestTicket(t)

	events := ticket.Events()
	events[0].Type = EventTicketClosed

	if ticket.Events()[0].Type != EventTicketCreated {
		t.Error("external slice mutation changed aggregate events")
	}

	ticket.ClearEvents()
	if len(ticket.Events()) != 0 {
		t.Error("ClearEvents() did not clear events")
	}
}

func newTestTicket(t *testing.T) *Ticket {
	t.Helper()

	ticket, err := NewTicket(validTicketParams(t), testTime())
	if err != nil {
		t.Fatalf("NewTicket() returned error: %v", err)
	}

	return ticket
}

func validTicketParams(t *testing.T) NewTicketParams {
	t.Helper()

	ticketID, err := NewTicketID()
	mustSucceed(t, err)

	requesterID, err := NewUserID()
	mustSucceed(t, err)

	categoryID, err := NewCategoryID()
	mustSucceed(t, err)

	return NewTicketParams{
		ID:          ticketID,
		RequesterID: requesterID,
		CategoryID:  categoryID,
		Title:       "VPN does not work after update",
		Description: "The VPN client cannot establish a connection.",
		Priority:    PriorityNormal,
		SLA: SLATarget{
			FirstResponse: 2 * time.Hour,
			Resolution:    24 * time.Hour,
		},
	}
}

func mustNewUserID(t *testing.T) UserID {
	t.Helper()

	id, err := NewUserID()
	mustSucceed(t, err)

	return id
}

func mustSucceed(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertStatus(t *testing.T, ticket *Ticket, want Status) {
	t.Helper()

	if got := ticket.Snapshot().Status; got != want {
		t.Fatalf("unexpected status: got %s want %s", got, want)
	}
}

func testTime() time.Time {
	return time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
}
