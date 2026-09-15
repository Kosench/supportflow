package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func (h WorkflowHandler) Reopen(
	ctx context.Context,
	command ReasonedWorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		workflowCommand(command),
		WorkflowReopen,
		func(ticket *domain.Ticket, repositories ports.Repositories, now time.Time) (bool, error) {
			sla, err := repositories.SLAPolicies.GetByPriority(
				ctx,
				ticket.Snapshot().Priority,
			)
			if err != nil {
				return false, err
			}

			if err := ticket.Reopen(
				command.Reason,
				sla.Resolution,
				now,
			); err != nil {
				return false, err
			}

			return true, nil
		},
	)
}
