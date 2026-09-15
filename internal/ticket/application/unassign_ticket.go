package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func (h WorkflowHandler) Unassign(
	ctx context.Context,
	command WorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		command,
		WorkflowUnassign,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			if err := ticket.Unassign(now); err != nil {
				return false, err
			}

			return true, nil
		},
	)
}
