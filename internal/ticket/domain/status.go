package domain

import (
	"fmt"
	"strings"
)

type Status string

const (
	StatusNew             Status = "NEW"
	StatusAssigned        Status = "ASSIGNED"
	StatusInProgress      Status = "IN_PROGRESS"
	StatusWaitingCustomer Status = "WAITING_CUSTOMER"
	StatusResolved        Status = "RESOLVED"
	StatusClosed          Status = "CLOSED"
	StatusReopened        Status = "REOPENED"
	StatusCancelled       Status = "CANCELLED"
)

func ParseStatus(raw string) (Status, error) {
	status := Status(strings.ToUpper(strings.TrimSpace(raw)))
	if !status.Valid() {
		return "", fmt.Errorf(
			"%w: unsupported status %q",
			ErrValidation,
			raw,
		)
	}

	return status, nil
}

func (status Status) Valid() bool {
	switch status {
	case StatusNew,
		StatusAssigned,
		StatusInProgress,
		StatusWaitingCustomer,
		StatusResolved,
		StatusClosed,
		StatusReopened,
		StatusCancelled:
		return true
	default:
		return false
	}
}

func (status Status) CanTransitionTo(next Status) bool {
	switch status {
	case StatusNew:
		return next == StatusAssigned || next == StatusCancelled
	case StatusAssigned:
		return next == StatusInProgress ||
			next == StatusNew ||
			next == StatusCancelled
	case StatusInProgress:
		return next == StatusWaitingCustomer ||
			next == StatusResolved
	case StatusWaitingCustomer:
		return next == StatusInProgress ||
			next == StatusResolved
	case StatusResolved:
		return next == StatusClosed ||
			next == StatusReopened
	case StatusClosed:
		return next == StatusReopened
	case StatusReopened:
		return next == StatusAssigned ||
			next == StatusInProgress
	case StatusCancelled:
		return false
	default:
		return false
	}
}
