package application

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func (h WorkflowHandler) RecordFirstResponse(
	ctx context.Context,
	command WorkflowCommand,
) (WorkflowResult, error) {
	return h.executor.execute(
		ctx,
		command,
		WorkflowRecordFirstResponse,
		func(ticket *domain.Ticket, _ ports.Repositories, now time.Time) (bool, error) {
			return ticket.MarkFirstResponded(now)
		},
	)
}
