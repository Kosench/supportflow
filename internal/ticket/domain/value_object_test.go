package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewTicketIDCreatesUUIDv7(t *testing.T) {
	id, err := NewTicketID()
	if err != nil {
		t.Fatalf("NewTicketID() returned error: %v", err)
	}

	if id.IsZero() {
		t.Fatal("NewTicketID() returned zero ID")
	}

	if uuid.UUID(id).Version() != 7 {
		t.Errorf(
			"unexpected UUID version: %d",
			uuid.UUID(id).Version(),
		)
	}
}

func TestParseTicketIDRequiresCanonicalForm(t *testing.T) {
	generated, err := NewTicketID()
	if err != nil {
		t.Fatalf("NewTicketID() returned error: %v", err)
	}

	parsed, err := ParseTicketID(generated.String())
	if err != nil {
		t.Fatalf("ParseTicketID() returned error: %v", err)
	}

	if parsed != generated {
		t.Errorf("parsed ID differs: got %s want %s", parsed, generated)
	}

	_, err = ParseTicketID("018f47d267b77c0c98b0b26dcd4f37df")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Priority
	}{
		{name: "low", raw: "low", want: PriorityLow},
		{name: "normal", raw: " NORMAL ", want: PriorityNormal},
		{name: "high", raw: "High", want: PriorityHigh},
		{name: "critical", raw: "CRITICAL", want: PriorityCritical},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParsePriority(test.raw)
			if err != nil {
				t.Fatalf("ParsePriority() returned error: %v", err)
			}

			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}

	_, err := ParsePriority("urgent")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestStatusTransitions(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{
			name: "new can be assigned",
			from: StatusNew,
			to:   StatusAssigned,
			want: true,
		},
		{
			name: "assigned can start",
			from: StatusAssigned,
			to:   StatusInProgress,
			want: true,
		},
		{
			name: "in progress can wait",
			from: StatusInProgress,
			to:   StatusWaitingCustomer,
			want: true,
		},
		{
			name: "closed can reopen",
			from: StatusClosed,
			to:   StatusReopened,
			want: true,
		},
		{
			name: "new cannot resolve",
			from: StatusNew,
			to:   StatusResolved,
			want: false,
		},
		{
			name: "cancelled is terminal",
			from: StatusCancelled,
			to:   StatusNew,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.from.CanTransitionTo(test.to)
			if got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
