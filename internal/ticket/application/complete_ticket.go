package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func (h WorkflowHandler) Resolve(
	ctx context.Context,
	command ReasonedWorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		workflowCommand(command),
		WorkflowResolve,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			if err := ticket.Resolve(command.Reason, now); err != nil {
				return false, err
			}

			return true, nil
		},
	)
}

func (h WorkflowHandler) Close(
	ctx context.Context,
	command ReasonedWorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		workflowCommand(command),
		WorkflowClose,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			if err := ticket.Close(command.Reason, now); err != nil {
				return false, err
			}

			return true, nil
		},
	)
}
