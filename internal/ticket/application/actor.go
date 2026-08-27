package application

import (
	"errors"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

var (
	ErrUnauthenticated     = errors.New("actor is not authenticated")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrCategoryUnavailable = errors.New("category is unavailable")
	ErrInvalidDependency   = errors.New("invalid application dependency")
)

type Role string

const (
	RoleUser     Role = "USER"
	RoleOperator Role = "OPERATOR"
	RoleManager  Role = "MANAGER"
	RoleAdmin    Role = "ADMIN"
)

func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleOperator, RoleManager, RoleAdmin:
		return true
	default:
		return false
	}
}

type Actor struct {
	UserID domain.UserID
	Role   Role
}

func (a Actor) Valid() error {
	if a.UserID.IsZero() || !a.Role.Valid() {
		return ErrUnauthenticated
	}

	return nil
}

type PermissionDeniedError struct {
	Operation string
	Role      Role
}

func (err PermissionDeniedError) Error() string {
	return fmt.Sprintf(
		"%s: operation=%s role=%s",
		ErrPermissionDenied,
		err.Operation,
		err.Role,
	)
}

func (err PermissionDeniedError) Unwrap() error {
	return ErrPermissionDenied
}

func deny(actor Actor, operation string) error {
	return PermissionDeniedError{
		Operation: operation,
		Role:      actor.Role,
	}
}

func authorizeRead(actor Actor, ticket *domain.Ticket) error {
	snapshot := ticket.Snapshot()

	switch actor.Role {
	case RoleUser:
		if snapshot.RequesterID == actor.UserID {
			return nil
		}
	case RoleOperator:
		if snapshot.AssigneeID != nil &&
			*snapshot.AssigneeID == actor.UserID {
			return nil
		}
	case RoleManager, RoleAdmin:
		return nil
	}

	return deny(actor, "ticket.read")
}

func authorizePriorityChange(
	actor Actor,
	ticket *domain.Ticket,
) error {
	snapshot := ticket.Snapshot()

	switch actor.Role {
	case RoleOperator:
		if snapshot.AssigneeID != nil &&
			*snapshot.AssigneeID == actor.UserID {
			return nil
		}
	case RoleManager, RoleAdmin:
		return nil
	}

	return deny(actor, "ticket.priority.change")
}

func authorizeCancel(actor Actor, ticket *domain.Ticket) error {
	snapshot := ticket.Snapshot()

	switch actor.Role {
	case RoleUser:
		if snapshot.RequesterID == actor.UserID {
			return nil
		}
	case RoleManager, RoleAdmin:
		return nil
	}

	return deny(actor, "ticket.cancel")
}
