package domain

import (
	"errors"
	"fmt"
)

var (
	ErrValidation        = errors.New("domain validation failed")
	ErrInvalidTransition = errors.New("invalid ticket status transition")
	ErrAssigneeRequired  = errors.New("ticket assignee is required")
	ErrNoChanges         = errors.New("operation does not change ticket")
	ErrInvalidTime       = errors.New("invalid operation time")
)

type TransitionError struct {
	From Status
	To   Status
}

func (err TransitionError) Error() string {
	return fmt.Sprintf(
		"%s: %s -> %s",
		ErrInvalidTransition,
		err.From,
		err.To,
	)
}

func (err TransitionError) Unwrap() error {
	return ErrInvalidTransition
}
