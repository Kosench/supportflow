package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type TicketID uuid.UUID
type UserID uuid.UUID
type CategoryID uuid.UUID

func NewTicketID() (TicketID, error) {
	value, err := newUUIDv7()
	if err != nil {
		return TicketID{}, fmt.Errorf("create ticket ID: %w", err)
	}

	return TicketID(value), nil
}

func NewUserID() (UserID, error) {
	value, err := newUUIDv7()
	if err != nil {
		return UserID{}, fmt.Errorf("create user ID: %w", err)
	}

	return UserID(value), nil
}

func NewCategoryID() (CategoryID, error) {
	value, err := newUUIDv7()
	if err != nil {
		return CategoryID{}, fmt.Errorf("create category ID: %w", err)
	}

	return CategoryID(value), nil
}

func ParseTicketID(raw string) (TicketID, error) {
	value, err := parseCanonicalUUID(raw)
	if err != nil {
		return TicketID{}, fmt.Errorf("parse ticket ID: %w", err)
	}

	return TicketID(value), nil
}

func ParseUserID(raw string) (UserID, error) {
	value, err := parseCanonicalUUID(raw)
	if err != nil {
		return UserID{}, fmt.Errorf("parse user ID: %w", err)
	}

	return UserID(value), nil
}

func ParseCategoryID(raw string) (CategoryID, error) {
	value, err := parseCanonicalUUID(raw)
	if err != nil {
		return CategoryID{}, fmt.Errorf("parse category ID: %w", err)
	}

	return CategoryID(value), nil
}

func (id TicketID) String() string {
	return uuid.UUID(id).String()
}

func (id UserID) String() string {
	return uuid.UUID(id).String()
}

func (id CategoryID) String() string {
	return uuid.UUID(id).String()
}

func (id TicketID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

func (id UserID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

func (id CategoryID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

func newUUIDv7() (uuid.UUID, error) {
	value, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate UUIDv7: %w", err)
	}

	return value, nil
}

func parseCanonicalUUID(raw string) (uuid.UUID, error) {
	value := strings.TrimSpace(raw)

	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %q", ErrValidation, raw)
	}

	if parsed == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: UUID must not be nil", ErrValidation)
	}

	if parsed.String() != strings.ToLower(value) {
		return uuid.Nil, fmt.Errorf(
			"%w: UUID must use canonical 36-character form",
			ErrValidation,
		)
	}

	return parsed, nil
}
