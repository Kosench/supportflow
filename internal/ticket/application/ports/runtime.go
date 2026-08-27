package ports

import (
	"time"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type Clock interface {
	Now() time.Time
}

type TicketIDGenerator interface {
	NewTicketID() (domain.TicketID, error)
}
