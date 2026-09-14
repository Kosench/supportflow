package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application"
	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func TestAutoAssignTicketAssignsBestCandidate(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))
	environment.operators.candidate = ports.OperatorCandidate{
		UserID:               operatorID,
		ActiveTicketCount:    1,
		MaxActiveTicketCount: 5,
	}

	handler, err := application.NewAutoAssignTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewAutoAssignTicketHandler() returned error: %v", err)
	}

	result, err := handler.Handle(
		context.Background(),
		application.AutoAssignTicketCommand{
			TicketID: ticketIDText,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if !result.Assigned || !result.Changed {
		t.Fatalf("unexpected assignment result: %+v", result)
	}

	if result.OperatorID == nil || *result.OperatorID != operatorIDText {
		t.Errorf("unexpected operator ID: %v", result.OperatorID)
	}

	if result.Ticket.Status != string(domain.StatusAssigned) {
		t.Errorf("unexpected status: %s", result.Ticket.Status)
	}

	if result.Ticket.Version != 2 {
		t.Errorf("unexpected version: %d", result.Ticket.Version)
	}

	if environment.operators.eligibilityCalls != 1 {
		t.Error("candidate eligibility must be rechecked")
	}

	if environment.operators.markCalls != 1 {
		t.Error("last_assigned_at must be updated")
	}
}

func TestAutoAssignTicketWithoutCandidateIsNotError(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))
	environment.operators.findErr = ports.ErrNoEligibleOperator

	handler, err := application.NewAutoAssignTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewAutoAssignTicketHandler() returned error: %v", err)
	}

	result, err := handler.Handle(
		context.Background(),
		application.AutoAssignTicketCommand{
			TicketID: ticketIDText,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if result.Assigned || result.Changed {
		t.Fatalf("unexpected assignment result: %+v", result)
	}

	if result.Ticket.Status != string(domain.StatusNew) {
		t.Errorf("unexpected status: %s", result.Ticket.Status)
	}
}

func TestAutoAssignTicketIsIdempotentAfterAssignment(t *testing.T) {
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

	handler, err := application.NewAutoAssignTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewAutoAssignTicketHandler() returned error: %v", err)
	}

	result, err := handler.Handle(
		context.Background(),
		application.AutoAssignTicketCommand{
			TicketID: ticketIDText,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if !result.Assigned || result.Changed {
		t.Fatalf("unexpected assignment result: %+v", result)
	}

	if environment.operators.findCalls != 0 {
		t.Error("candidate search must not run for assigned ticket")
	}
}

func TestClaimTicketByEligibleOperator(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))
	environment.operators.candidate = ports.OperatorCandidate{
		UserID:               operatorID,
		ActiveTicketCount:    0,
		MaxActiveTicketCount: 2,
	}

	handler, err := application.NewClaimTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewClaimTicketHandler() returned error: %v", err)
	}

	result, err := handler.Handle(
		context.Background(),
		application.ClaimTicketCommand{
			Actor: application.Actor{
				UserID: operatorID,
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			ExpectedVersion: 1,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if !result.Assigned || !result.Changed {
		t.Fatalf("unexpected assignment result: %+v", result)
	}
}

func TestClaimTicketDoesNotRevealStateOrVersionToIneligibleOperator(
	t *testing.T,
) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	ticket := newStoredTicket(t, requesterID)

	if err := ticket.Assign(
		mustUserID(t, managerIDText),
		baseTime.Add(time.Minute),
	); err != nil {
		t.Fatalf("Assign() returned error: %v", err)
	}
	environment.tickets.put(ticket)
	environment.operators.lockErr = ports.ErrNoEligibleOperator

	handler, err := application.NewClaimTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(2 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewClaimTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.ClaimTicketCommand{
			Actor: application.Actor{
				UserID: operatorID,
				Role:   application.RoleOperator,
			},
			TicketID:        ticketIDText,
			ExpectedVersion: 99,
		},
	)
	requireErrorIs(t, err, application.ErrOperatorUnavailable)
}

func TestForceAssignTicketByManagerBypassesEligibility(t *testing.T) {
	environment := newAssignmentEnvironment()
	requesterID := mustUserID(t, requesterIDText)
	operatorID := mustUserID(t, operatorIDText)
	environment.tickets.put(newStoredTicket(t, requesterID))
	environment.operators.eligible = false

	handler, err := application.NewForceAssignTicketHandler(
		environment.unit,
		fixedClock{value: baseTime.Add(time.Minute)},
	)
	if err != nil {
		t.Fatalf("NewForceAssignTicketHandler() returned error: %v", err)
	}

	result, err := handler.Handle(
		context.Background(),
		application.ForceAssignTicketCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, managerIDText),
				Role:   application.RoleManager,
			},
			TicketID:         ticketIDText,
			TargetOperatorID: operatorIDText,
			ExpectedVersion:  1,
		},
	)
	if err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}

	if !result.Assigned || !result.Changed {
		t.Fatalf("unexpected assignment result: %+v", result)
	}

	if environment.operators.activeLockCalls != 1 {
		t.Error("target operator must be locked")
	}

	if environment.operators.markedOperatorID != operatorID {
		t.Errorf(
			"unexpected marked operator: %s",
			environment.operators.markedOperatorID,
		)
	}

	if environment.operators.eligibilityCalls != 0 {
		t.Error("force assignment must not check normal eligibility")
	}
}

func TestForceAssignTicketRejectsUserBeforeTransaction(t *testing.T) {
	environment := newAssignmentEnvironment()

	handler, err := application.NewForceAssignTicketHandler(
		environment.unit,
		fixedClock{value: baseTime},
	)
	if err != nil {
		t.Fatalf("NewForceAssignTicketHandler() returned error: %v", err)
	}

	_, err = handler.Handle(
		context.Background(),
		application.ForceAssignTicketCommand{
			Actor: application.Actor{
				UserID: mustUserID(t, requesterIDText),
				Role:   application.RoleUser,
			},
			TicketID:         ticketIDText,
			TargetOperatorID: operatorIDText,
			ExpectedVersion:  1,
		},
	)
	requireErrorIs(t, err, application.ErrPermissionDenied)

	if environment.unit.calls != 0 {
		t.Error("transaction must not start for forbidden command")
	}
}
