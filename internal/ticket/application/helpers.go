package application

import (
	"context"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

func validateExpectedVersion(expectedVersion uint64) error {
	if expectedVersion == 0 {
		return fmt.Errorf(
			"%w: expected version must be positive",
			domain.ErrValidation,
		)
	}

	return nil
}

func loadTicketForUpdate(ctx context.Context, repository ports.TicketRepository, ticketID domain.TicketID,
) (*domain.Ticket, uint64, error) {
	ticket, err := repository.GetByID(ctx, ticketID)
	if err != nil {
		return nil, 0, err
	}

	return ticket, ticket.Snapshot().Version, nil
}

func checkExpectedVersion(
	ticketID domain.TicketID,
	expectedVersion uint64,
	actualVersion uint64,
) error {
	if actualVersion != expectedVersion {
		return ports.VersionConflictError{
			TicketID: ticketID,
			Expected: expectedVersion,
			Actual:   actualVersion,
		}
	}

	return nil
}
