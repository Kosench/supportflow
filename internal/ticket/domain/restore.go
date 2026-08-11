package domain

import (
	"fmt"
	"strings"
	"time"
)

func RestoreTicket(snapshot TicketSnapshot) (*Ticket, error) {
	if snapshot.ID.IsZero() {
		return nil, fmt.Errorf("%w: ticket ID is required", ErrValidation)
	}

	if snapshot.RequesterID.IsZero() {
		return nil, fmt.Errorf("%w: requester ID is required", ErrValidation)
	}

	if snapshot.CategoryID.IsZero() {
		return nil, fmt.Errorf("%w: category ID is required", ErrValidation)
	}

	title, err := requiredText(
		"title",
		snapshot.Title,
		minTitleLength,
		maxTitleLength,
	)
	if err != nil {
		return nil, err
	}

	description, err := requiredText(
		"description",
		snapshot.Description,
		minDescriptionLength,
		maxDescriptionLength,
	)
	if err != nil {
		return nil, err
	}

	if !snapshot.Priority.Valid() {
		return nil, fmt.Errorf("%w: priority is invalid", ErrValidation)
	}

	if !snapshot.Status.Valid() {
		return nil, fmt.Errorf("%w: status is invalid", ErrValidation)
	}

	if snapshot.Version == 0 {
		return nil, fmt.Errorf("%w: version must be positive", ErrValidation)
	}

	createdAt, err := requiredTime("created_at", snapshot.CreatedAt)
	if err != nil {
		return nil, err
	}

	updatedAt, err := requiredTime("updated_at", snapshot.UpdatedAt)
	if err != nil {
		return nil, err
	}

	resolutionStartedAt, err := requiredTime(
		"resolution_started_at",
		snapshot.ResolutionStartedAt,
	)
	if err != nil {
		return nil, err
	}

	firstResponseDeadlineAt, err := requiredTime(
		"first_response_deadline_at",
		snapshot.FirstResponseDeadlineAt,
	)
	if err != nil {
		return nil, err
	}

	resolutionDeadlineAt, err := requiredTime(
		"resolution_deadline_at",
		snapshot.ResolutionDeadlineAt,
	)
	if err != nil {
		return nil, err
	}

	firstRespondedAt, err := optionalTime(
		"first_responded_at",
		snapshot.FirstRespondedAt,
	)
	if err != nil {
		return nil, err
	}

	resolvedAt, err := optionalTime("resolved_at", snapshot.ResolvedAt)
	if err != nil {
		return nil, err
	}

	closedAt, err := optionalTime("closed_at", snapshot.ClosedAt)
	if err != nil {
		return nil, err
	}

	if updatedAt.Before(createdAt) {
		return nil, invalidStoredTime("updated_at", "created_at")
	}

	if resolutionStartedAt.Before(createdAt) {
		return nil, invalidStoredTime(
			"resolution_started_at",
			"created_at",
		)
	}

	if !firstResponseDeadlineAt.After(createdAt) {
		return nil, invalidStoredTime(
			"first_response_deadline_at",
			"created_at",
		)
	}

	if !resolutionDeadlineAt.After(resolutionStartedAt) {
		return nil, invalidStoredTime(
			"resolution_deadline_at",
			"resolution_started_at",
		)
	}

	for field, value := range map[string]*time.Time{
		"first_responded_at": firstRespondedAt,
		"resolved_at":        resolvedAt,
	} {
		if value != nil && value.Before(createdAt) {
			return nil, invalidStoredTime(field, "created_at")
		}
	}

	if closedAt != nil {
		if resolvedAt == nil || closedAt.Before(*resolvedAt) {
			return nil, invalidStoredTime("closed_at", "resolved_at")
		}
	}

	if err := validateStoredAssignee(
		snapshot.Status,
		snapshot.AssigneeID,
	); err != nil {
		return nil, err
	}

	waitingReason, err := validateStoredWaitingReason(
		snapshot.Status,
		snapshot.WaitingReason,
	)
	if err != nil {
		return nil, err
	}

	resolution, err := validateStoredResolution(
		snapshot.Status,
		snapshot.Resolution,
		resolvedAt,
		closedAt,
	)
	if err != nil {
		return nil, err
	}

	return &Ticket{
		id:                      snapshot.ID,
		requesterID:             snapshot.RequesterID,
		categoryID:              snapshot.CategoryID,
		assigneeID:              cloneUserID(snapshot.AssigneeID),
		title:                   title,
		description:             description,
		priority:                snapshot.Priority,
		status:                  snapshot.Status,
		waitingReason:           waitingReason,
		resolution:              resolution,
		version:                 snapshot.Version,
		createdAt:               createdAt,
		updatedAt:               updatedAt,
		resolutionStartedAt:     resolutionStartedAt,
		firstResponseDeadlineAt: firstResponseDeadlineAt,
		resolutionDeadlineAt:    resolutionDeadlineAt,
		firstRespondedAt:        firstRespondedAt,
		resolvedAt:              resolvedAt,
		closedAt:                closedAt,
		events:                  nil,
	}, nil
}

func validateStoredAssignee(status Status, assigneeID *UserID) error {
	switch status {
	case StatusAssigned, StatusInProgress, StatusWaitingCustomer:
		if assigneeID == nil || assigneeID.IsZero() {
			return ErrAssigneeRequired
		}
	case StatusNew, StatusCancelled:
		if assigneeID != nil {
			return fmt.Errorf(
				"%w: status %s must not have assignee",
				ErrValidation,
				status,
			)
		}
	default:
		if assigneeID != nil && assigneeID.IsZero() {
			return fmt.Errorf(
				"%w: assignee ID must not be zero",
				ErrValidation,
			)
		}
	}

	return nil
}

func validateStoredWaitingReason(
	status Status,
	value string,
) (string, error) {
	if status == StatusWaitingCustomer {
		return requiredText(
			"waiting reason",
			value,
			minReasonLength,
			maxReasonLength,
		)
	}

	if value != "" {
		return "", fmt.Errorf(
			"%w: status %s must not have waiting reason",
			ErrValidation,
			status,
		)
	}

	return "", nil
}

func validateStoredResolution(
	status Status,
	value string,
	resolvedAt *time.Time,
	closedAt *time.Time,
) (string, error) {
	if status == StatusResolved || status == StatusClosed {
		if resolvedAt == nil {
			return "", fmt.Errorf(
				"%w: status %s requires resolved_at",
				ErrValidation,
				status,
			)
		}

		resolution, err := requiredText(
			"resolution",
			value,
			minReasonLength,
			maxResolutionLength,
		)
		if err != nil {
			return "", err
		}

		if status == StatusClosed && closedAt == nil {
			return "", fmt.Errorf(
				"%w: CLOSED requires closed_at",
				ErrValidation,
			)
		}

		if status == StatusResolved && closedAt != nil {
			return "", fmt.Errorf(
				"%w: RESOLVED must not have closed_at",
				ErrValidation,
			)
		}

		return resolution, nil
	}

	if strings.TrimSpace(value) != "" || resolvedAt != nil || closedAt != nil {
		return "", fmt.Errorf(
			"%w: status %s must not contain resolution timestamps",
			ErrValidation,
			status,
		)
	}

	return "", nil
}

func requiredTime(field string, value time.Time) (time.Time, error) {
	if value.IsZero() {
		return time.Time{}, fmt.Errorf(
			"%w: %s must not be zero",
			ErrInvalidTime,
			field,
		)
	}

	return value.UTC(), nil
}

func optionalTime(
	field string,
	value *time.Time,
) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	normalized, err := requiredTime(field, *value)
	if err != nil {
		return nil, err
	}

	return timePointer(normalized), nil
}

func invalidStoredTime(field, reference string) error {
	return fmt.Errorf(
		"%w: %s is inconsistent with %s",
		ErrInvalidTime,
		field,
		reference,
	)
}
