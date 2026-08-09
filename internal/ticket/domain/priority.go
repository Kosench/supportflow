package domain

import (
	"fmt"
	"strings"
	"time"
)

type Priority string

const (
	PriorityLow      Priority = "LOW"
	PriorityNormal   Priority = "NORMAL"
	PriorityHigh     Priority = "HIGH"
	PriorityCritical Priority = "CRITICAL"
)

func ParsePriority(raw string) (Priority, error) {
	priority := Priority(strings.ToUpper(strings.TrimSpace(raw)))
	if !priority.Valid() {
		return "", fmt.Errorf(
			"%w: unsupported priority %q",
			ErrValidation,
			raw,
		)
	}

	return priority, nil
}

func (priority Priority) Valid() bool {
	switch priority {
	case PriorityLow,
		PriorityNormal,
		PriorityHigh,
		PriorityCritical:
		return true
	default:
		return false
	}
}

type SLATarget struct {
	FirstResponse time.Duration
	Resolution    time.Duration
}

func (target SLATarget) Validate() error {
	if target.FirstResponse <= 0 {
		return fmt.Errorf(
			"%w: first response SLA must be positive",
			ErrValidation,
		)
	}

	if target.Resolution <= 0 {
		return fmt.Errorf(
			"%w: resolution SLA must be positive",
			ErrValidation,
		)
	}

	if target.Resolution < target.FirstResponse {
		return fmt.Errorf(
			"%w: resolution SLA must not be shorter than first response SLA",
			ErrValidation,
		)
	}

	return nil
}
