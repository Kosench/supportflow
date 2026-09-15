package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func (h WorkflowHandler) WaitForCustomer(
	ctx context.Context,
	command ReasonedWorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		workflowCommand(command),
		WorkflowWaitForCustomer,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			if err := ticket.WaitForCustomer(command.Reason, now); err != nil {
				return false, err
			}
			return true, nil
		},
	)
}

func (h WorkflowHandler) ResumeProgress(
	ctx context.Context,
	command WorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		command,
		WorkflowResumeProgress,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			if err := ticket.ResumeProgress(now); err != nil {
				return false, err
			}

			return true, nil
		},
	)
}
