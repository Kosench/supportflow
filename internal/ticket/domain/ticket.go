package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minTitleLength       = 5
	maxTitleLength       = 200
	minDescriptionLength = 10
	maxDescriptionLength = 10_000
	minReasonLength      = 3
	maxReasonLength      = 1_000
	maxResolutionLength  = 4_000
)

type NewTicketParams struct {
	ID          TicketID
	RequesterID UserID
	CategoryID  CategoryID
	Title       string
	Description string
	Priority    Priority
	SLA         SLATarget
}

type Ticket struct {
	id          TicketID
	requesterID UserID
	categoryID  CategoryID
	assigneeID  *UserID

	title         string
	description   string
	priority      Priority
	status        Status
	waitingReason string
	resolution    string

	version uint64

	createdAt               time.Time
	updatedAt               time.Time
	resolutionStartedAt     time.Time
	firstResponseDeadlineAt time.Time
	resolutionDeadlineAt    time.Time
	firstRespondedAt        *time.Time
	resolvedAt              *time.Time
	closedAt                *time.Time

	events []DomainEvent
}

type TicketSnapshot struct {
	ID          TicketID
	RequesterID UserID
	CategoryID  CategoryID
	AssigneeID  *UserID

	Title         string
	Description   string
	Priority      Priority
	Status        Status
	WaitingReason string
	Resolution    string

	Version uint64

	CreatedAt               time.Time
	UpdatedAt               time.Time
	ResolutionStartedAt     time.Time
	FirstResponseDeadlineAt time.Time
	ResolutionDeadlineAt    time.Time
	FirstRespondedAt        *time.Time
	ResolvedAt              *time.Time
	ClosedAt                *time.Time
}

func NewTicket(params NewTicketParams, now time.Time) (*Ticket, error) {
	if params.ID.IsZero() {
		return nil, fmt.Errorf("%w: ticket ID is required", ErrValidation)
	}

	if params.RequesterID.IsZero() {
		return nil, fmt.Errorf("%w: requester ID is required", ErrValidation)
	}

	if params.CategoryID.IsZero() {
		return nil, fmt.Errorf("%w: category ID is required", ErrValidation)
	}

	title, err := requiredText(
		"title",
		params.Title,
		minTitleLength,
		maxTitleLength,
	)
	if err != nil {
		return nil, err
	}

	description, err := requiredText(
		"description",
		params.Description,
		minDescriptionLength,
		maxDescriptionLength,
	)
	if err != nil {
		return nil, err
	}

	if !params.Priority.Valid() {
		return nil, fmt.Errorf(
			"%w: priority is invalid",
			ErrValidation,
		)
	}

	if err := params.SLA.Validate(); err != nil {
		return nil, err
	}

	createdAt, err := initialTime(now)
	if err != nil {
		return nil, err
	}

	ticket := &Ticket{
		id:                      params.ID,
		requesterID:             params.RequesterID,
		categoryID:              params.CategoryID,
		title:                   title,
		description:             description,
		priority:                params.Priority,
		status:                  StatusNew,
		version:                 1,
		createdAt:               createdAt,
		updatedAt:               createdAt,
		resolutionStartedAt:     createdAt,
		firstResponseDeadlineAt: createdAt.Add(params.SLA.FirstResponse),
		resolutionDeadlineAt:    createdAt.Add(params.SLA.Resolution),
	}

	ticket.recordEvent(
		EventTicketCreated,
		TicketCreatedData{
			RequesterID:             ticket.requesterID,
			CategoryID:              ticket.categoryID,
			Priority:                ticket.priority,
			Status:                  ticket.status,
			Title:                   ticket.title,
			FirstResponseDeadlineAt: ticket.firstResponseDeadlineAt,
			ResolutionDeadlineAt:    ticket.resolutionDeadlineAt,
		},
		createdAt,
	)

	return ticket, nil
}

func (ticket *Ticket) Assign(assigneeID UserID, now time.Time) error {
	if assigneeID.IsZero() {
		return fmt.Errorf("%w: assignee ID is required", ErrValidation)
	}

	switch ticket.status {
	case StatusNew,
		StatusReopened,
		StatusAssigned,
		StatusInProgress,
		StatusWaitingCustomer:
	default:
		return TransitionError{
			From: ticket.status,
			To:   StatusAssigned,
		}
	}

	if ticket.assigneeID != nil && *ticket.assigneeID == assigneeID {
		return ErrNoChanges
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousAssigneeID := cloneUserID(ticket.assigneeID)
	previousStatus := ticket.status

	assigneeCopy := assigneeID
	ticket.assigneeID = &assigneeCopy

	if ticket.status == StatusNew || ticket.status == StatusReopened {
		ticket.status = StatusAssigned
	}

	ticket.touch(changedAt)

	if previousStatus != ticket.status {
		ticket.recordStatusChanged(
			previousStatus,
			ticket.status,
			"ticket assigned",
			changedAt,
		)
	}

	ticket.recordEvent(
		EventTicketAssigned,
		TicketAssignedData{
			AssigneeID:         assigneeID,
			PreviousAssigneeID: previousAssigneeID,
		},
		changedAt,
	)

	return nil
}

func (ticket *Ticket) Unassign(now time.Time) error {
	if ticket.status != StatusAssigned {
		return TransitionError{
			From: ticket.status,
			To:   StatusNew,
		}
	}

	if ticket.assigneeID == nil {
		return ErrAssigneeRequired
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	previousAssigneeID := *ticket.assigneeID

	ticket.assigneeID = nil
	ticket.status = StatusNew
	ticket.touch(changedAt)

	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		"ticket unassigned",
		changedAt,
	)
	ticket.recordEvent(
		EventTicketUnassigned,
		TicketUnassignedData{
			PreviousAssigneeID: previousAssigneeID,
		},
		changedAt,
	)

	return nil
}

func (ticket *Ticket) StartProgress(now time.Time) error {
	if ticket.status != StatusAssigned && ticket.status != StatusReopened {
		return TransitionError{
			From: ticket.status,
			To:   StatusInProgress,
		}
	}

	if ticket.assigneeID == nil {
		return ErrAssigneeRequired
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusInProgress
	ticket.touch(changedAt)
	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		"work started",
		changedAt,
	)

	return nil
}

func (ticket *Ticket) WaitForCustomer(reason string, now time.Time) error {
	if !ticket.status.CanTransitionTo(StatusWaitingCustomer) {
		return TransitionError{
			From: ticket.status,
			To:   StatusWaitingCustomer,
		}
	}

	waitingReason, err := requiredText(
		"waiting reason",
		reason,
		minReasonLength,
		maxReasonLength,
	)
	if err != nil {
		return err
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusWaitingCustomer
	ticket.waitingReason = waitingReason
	ticket.touch(changedAt)
	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		waitingReason,
		changedAt,
	)

	return nil
}

func (ticket *Ticket) ResumeProgress(now time.Time) error {
	if ticket.status != StatusWaitingCustomer {
		return TransitionError{
			From: ticket.status,
			To:   StatusInProgress,
		}
	}

	if ticket.assigneeID == nil {
		return ErrAssigneeRequired
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusInProgress
	ticket.waitingReason = ""
	ticket.touch(changedAt)
	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		"customer wait completed",
		changedAt,
	)

	return nil
}

func (ticket *Ticket) Resolve(solution string, now time.Time) error {
	if !ticket.status.CanTransitionTo(StatusResolved) {
		return TransitionError{
			From: ticket.status,
			To:   StatusResolved,
		}
	}

	resolution, err := requiredText(
		"resolution",
		solution,
		minReasonLength,
		maxResolutionLength,
	)
	if err != nil {
		return err
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusResolved
	ticket.waitingReason = ""
	ticket.resolution = resolution
	ticket.resolvedAt = timePointer(changedAt)
	ticket.closedAt = nil
	ticket.touch(changedAt)

	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		resolution,
		changedAt,
	)
	ticket.recordEvent(EventTicketResolved, struct{}{}, changedAt)

	return nil
}

func (ticket *Ticket) Close(reason string, now time.Time) error {
	if !ticket.status.CanTransitionTo(StatusClosed) {
		return TransitionError{
			From: ticket.status,
			To:   StatusClosed,
		}
	}

	closeReason, err := requiredText(
		"close reason",
		reason,
		minReasonLength,
		maxReasonLength,
	)
	if err != nil {
		return err
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusClosed
	ticket.closedAt = timePointer(changedAt)
	ticket.touch(changedAt)

	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		closeReason,
		changedAt,
	)
	ticket.recordEvent(EventTicketClosed, struct{}{}, changedAt)

	return nil
}

func (ticket *Ticket) Reopen(
	reason string,
	resolutionWindow time.Duration,
	now time.Time,
) error {
	if !ticket.status.CanTransitionTo(StatusReopened) {
		return TransitionError{
			From: ticket.status,
			To:   StatusReopened,
		}
	}

	reopenReason, err := requiredText(
		"reopen reason",
		reason,
		minReasonLength,
		maxReasonLength,
	)
	if err != nil {
		return err
	}

	if resolutionWindow <= 0 {
		return fmt.Errorf(
			"%w: resolution window must be positive",
			ErrValidation,
		)
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	ticket.status = StatusReopened
	ticket.resolution = ""
	ticket.resolvedAt = nil
	ticket.closedAt = nil
	ticket.resolutionStartedAt = changedAt
	ticket.resolutionDeadlineAt = ticket.resolutionStartedAt.Add(
		resolutionWindow,
	)
	ticket.touch(changedAt)

	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		reopenReason,
		changedAt,
	)
	ticket.recordEvent(
		EventTicketReopened,
		TicketReopenedData{
			Reason:               reopenReason,
			ResolutionDeadlineAt: ticket.resolutionDeadlineAt,
		},
		changedAt,
	)

	return nil
}

func (ticket *Ticket) Cancel(reason string, now time.Time) error {
	if !ticket.status.CanTransitionTo(StatusCancelled) {
		return TransitionError{
			From: ticket.status,
			To:   StatusCancelled,
		}
	}

	cancelReason, err := requiredText(
		"cancel reason",
		reason,
		minReasonLength,
		maxReasonLength,
	)
	if err != nil {
		return err
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousStatus := ticket.status
	previousAssigneeID := cloneUserID(ticket.assigneeID)
	ticket.status = StatusCancelled
	ticket.assigneeID = nil
	ticket.waitingReason = ""
	ticket.touch(changedAt)
	ticket.recordStatusChanged(
		previousStatus,
		ticket.status,
		cancelReason,
		changedAt,
	)

	if previousAssigneeID != nil {
		ticket.recordEvent(
			EventTicketUnassigned,
			TicketUnassignedData{
				PreviousAssigneeID: *previousAssigneeID,
			},
			changedAt,
		)
	}

	return nil
}

func (ticket *Ticket) ChangePriority(
	priority Priority,
	reason string,
	sla SLATarget,
	now time.Time,
) error {
	switch ticket.status {
	case StatusNew,
		StatusAssigned,
		StatusInProgress,
		StatusWaitingCustomer,
		StatusReopened:
	default:
		return fmt.Errorf(
			"%w: priority cannot be changed in status %s",
			ErrInvalidTransition,
			ticket.status,
		)
	}

	if !priority.Valid() {
		return fmt.Errorf("%w: priority is invalid", ErrValidation)
	}

	if priority == ticket.priority {
		return ErrNoChanges
	}

	changeReason, err := requiredText(
		"priority change reason",
		reason,
		minReasonLength,
		maxReasonLength,
	)
	if err != nil {
		return err
	}

	if err := sla.Validate(); err != nil {
		return err
	}

	changedAt, err := ticket.mutationTime(now)
	if err != nil {
		return err
	}

	previousPriority := ticket.priority
	ticket.priority = priority

	if ticket.firstRespondedAt == nil {
		ticket.firstResponseDeadlineAt = ticket.createdAt.Add(
			sla.FirstResponse,
		)
	}

	if ticket.resolvedAt == nil {
		ticket.resolutionDeadlineAt = ticket.resolutionStartedAt.Add(
			sla.Resolution,
		)
	}

	ticket.touch(changedAt)
	ticket.recordEvent(
		EventTicketPriorityChanged,
		TicketPriorityChangedData{
			From:                    previousPriority,
			To:                      ticket.priority,
			Reason:                  changeReason,
			FirstResponseDeadlineAt: ticket.firstResponseDeadlineAt,
			ResolutionDeadlineAt:    ticket.resolutionDeadlineAt,
		},
		changedAt,
	)

	return nil
}

func (ticket *Ticket) MarkFirstResponded(now time.Time) (bool, error) {
	if ticket.firstRespondedAt != nil {
		return false, nil
	}

	switch ticket.status {
	case StatusAssigned,
		StatusInProgress,
		StatusWaitingCustomer:
	default:
		return false, fmt.Errorf(
			"%w: first response cannot be recorded in status %s",
			ErrInvalidTransition,
			ticket.status,
		)
	}

	if ticket.assigneeID == nil {
		return false, ErrAssigneeRequired
	}

	respondedAt, err := ticket.mutationTime(now)
	if err != nil {
		return false, err
	}

	ticket.firstRespondedAt = timePointer(respondedAt)
	ticket.touch(respondedAt)
	ticket.recordEvent(
		EventTicketFirstResponded,
		TicketFirstRespondedData{
			RespondedAt: respondedAt,
		},
		respondedAt,
	)

	return true, nil
}

func (ticket *Ticket) Snapshot() TicketSnapshot {
	return TicketSnapshot{
		ID:                      ticket.id,
		RequesterID:             ticket.requesterID,
		CategoryID:              ticket.categoryID,
		AssigneeID:              cloneUserID(ticket.assigneeID),
		Title:                   ticket.title,
		Description:             ticket.description,
		Priority:                ticket.priority,
		Status:                  ticket.status,
		WaitingReason:           ticket.waitingReason,
		Resolution:              ticket.resolution,
		Version:                 ticket.version,
		CreatedAt:               ticket.createdAt,
		UpdatedAt:               ticket.updatedAt,
		ResolutionStartedAt:     ticket.resolutionStartedAt,
		FirstResponseDeadlineAt: ticket.firstResponseDeadlineAt,
		ResolutionDeadlineAt:    ticket.resolutionDeadlineAt,
		FirstRespondedAt:        cloneTime(ticket.firstRespondedAt),
		ResolvedAt:              cloneTime(ticket.resolvedAt),
		ClosedAt:                cloneTime(ticket.closedAt),
	}
}

func (ticket *Ticket) Events() []DomainEvent {
	return slices.Clone(ticket.events)
}

func (ticket *Ticket) ClearEvents() {
	ticket.events = nil
}

func (ticket *Ticket) touch(now time.Time) {
	ticket.version++
	ticket.updatedAt = now
}

func (ticket *Ticket) recordStatusChanged(
	from Status,
	to Status,
	reason string,
	occurredAt time.Time,
) {
	ticket.recordEvent(
		EventTicketStatusChanged,
		TicketStatusChangedData{
			From:   from,
			To:     to,
			Reason: reason,
		},
		occurredAt,
	)
}

func (ticket *Ticket) recordEvent(
	eventType EventType,
	data any,
	occurredAt time.Time,
) {
	ticket.events = append(
		ticket.events,
		DomainEvent{
			Type:             eventType,
			AggregateID:      ticket.id,
			AggregateVersion: ticket.version,
			OccurredAt:       occurredAt,
			Data:             data,
		},
	)
}

func (ticket *Ticket) mutationTime(now time.Time) (time.Time, error) {
	if now.IsZero() {
		return time.Time{}, fmt.Errorf(
			"%w: operation time must not be zero",
			ErrInvalidTime,
		)
	}

	value := now.UTC()
	if value.Before(ticket.updatedAt) {
		return time.Time{}, fmt.Errorf(
			"%w: operation time %s is before updated_at %s",
			ErrInvalidTime,
			value.Format(time.RFC3339Nano),
			ticket.updatedAt.Format(time.RFC3339Nano),
		)
	}

	return value, nil
}

func initialTime(now time.Time) (time.Time, error) {
	if now.IsZero() {
		return time.Time{}, fmt.Errorf(
			"%w: creation time must not be zero",
			ErrInvalidTime,
		)
	}

	return now.UTC(), nil
}

func requiredText(
	field string,
	value string,
	minLength int,
	maxLength int,
) (string, error) {
	normalized := strings.TrimSpace(value)
	length := utf8.RuneCountInString(normalized)

	if length < minLength || length > maxLength {
		return "", fmt.Errorf(
			"%w: %s length must be between %d and %d characters",
			ErrValidation,
			field,
			minLength,
			maxLength,
		)
	}

	return normalized, nil
}

func timePointer(value time.Time) *time.Time {
	copyValue := value
	return &copyValue
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	return timePointer(*value)
}

func cloneUserID(value *UserID) *UserID {
	if value == nil {
		return nil
	}

	copyValue := *value
	return &copyValue
}
