package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewWritesStructuredLog(t *testing.T) {
	var output bytes.Buffer

	log, err := New(&output, Config{
		Level:       "info",
		Pretty:      false,
		Service:     "ticket-service",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	log.Info().
		Str("ticket_id", "ticket-123").
		Msg("ticket created")

	var event map[string]any
	if err := json.Unmarshal(
		[]byte(strings.TrimSpace(output.String())),
		&event,
	); err != nil {
		t.Fatalf("decode log as JSON: %v", err)
	}

	if event["level"] != "info" {
		t.Errorf("unexpected level: %v", event["level"])
	}

	if event["service"] != "ticket-service" {
		t.Errorf("unexpected service: %v", event["service"])
	}

	if event["environment"] != "test" {
		t.Errorf("unexpected environment: %v", event["environment"])
	}

	if event["ticket_id"] != "ticket-123" {
		t.Errorf("unexpected ticket ID: %v", event["ticket_id"])
	}

	if event["message"] != "ticket created" {
		t.Errorf("unexpected message: %v", event["message"])
	}

	if _, exists := event["time"]; !exists {
		t.Error("log does not contain time")
	}
}

func TestNewRejectsUnknownLevel(t *testing.T) {
	var output bytes.Buffer

	_, err := New(&output, Config{
		Level:       "verbose",
		Service:     "ticket-service",
		Environment: "test",
	})
	if err == nil {
		t.Fatal("New() error is nil")
	}
}

func TestNewRejectsNilOutput(t *testing.T) {
	_, err := New(nil, Config{
		Level:       "info",
		Service:     "ticket-service",
		Environment: "test",
	})
	if err == nil {
		t.Fatal("New() error is nil")
	}
}
