package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type WorkflowCommand struct {
	Actor           Actor
	TicketID        string
	ExpectedVersion uint64
}

type ReasonedWorkflowCommand struct {
	Actor           Actor
	TicketID        string
	Reason          string
	ExpectedVersion uint64
}

type WorkflowResult struct {
	Ticket  TicketView
	Changed bool
}

type WorkflowHandler struct {
	executor workflowExecutor
}

type workflowExecutor struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

type workflowMutation func(
	ticket *domain.Ticket,
	repositories ports.Repositories,
	now time.Time,
) (bool, error)

func NewWorkflowHandler(unitOfWork ports.UnitOfWork, clock ports.Clock) (WorkflowHandler, error) {
	if unitOfWork == nil || clock == nil {
		return WorkflowHandler{}, ErrInvalidDependency
	}

	return WorkflowHandler{
		executor: workflowExecutor{
			unitOfWork: unitOfWork,
			clock:      clock,
		},
	}, nil
}

func (executor workflowExecutor) execute(
	ctx context.Context,
	command WorkflowCommand,
	action WorkflowAction,
	mutation workflowMutation,
) (WorkflowResult, error) {
	if err := command.Actor.Valid(); err != nil {
		return WorkflowResult{}, err
	}

	if err := validateExpectedVersion(command.ExpectedVersion); err != nil {
		return WorkflowResult{}, err
	}

	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return WorkflowResult{}, err
	}

	var result WorkflowResult
	err = executor.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, err := repositories.Tickets.GetByIDForUpdate(ctx, ticketID)
			if err != nil {
				return err
			}

			snapshot := ticket.Snapshot()
			if err := authorizeWorkFlow(command.Actor, action, snapshot); err != nil {
				return err
			}

			if err := checkExpectedVersion(ticketID, command.ExpectedVersion, snapshot.Version); err != nil {
				return err
			}

			changed, err := mutation(ticket, repositories, executor.clock.Now())
			if err != nil {
				return err
			}

			if changed {
				if err := repositories.Tickets.Update(
					ctx, ticket, command.ExpectedVersion,
				); err != nil {
					return err
				}
			}

			result = WorkflowResult{
				Ticket:  newTicketView(ticket),
				Changed: changed,
			}
			return nil
		},
	)
	if err != nil {
		return WorkflowResult{}, err
	}

	return result, nil
}

func workflowCommand(
	command ReasonedWorkflowCommand,
) WorkflowCommand {
	return WorkflowCommand{
		Actor:           command.Actor,
		TicketID:        command.TicketID,
		ExpectedVersion: command.ExpectedVersion,
	}
}
