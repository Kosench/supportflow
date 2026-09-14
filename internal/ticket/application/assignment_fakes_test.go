package application_test

import (
	"context"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type fakeOperatorRepository struct {
	candidate ports.OperatorCandidate
	findErr   error
	lockErr   error
	activeErr error

	eligible    bool
	eligibleErr error
	markErr     error

	findCalls        int
	lockCalls        int
	activeLockCalls  int
	eligibilityCalls int
	markCalls        int
	markedOperatorID domain.UserID
	markedAt         time.Time
}

func (repository *fakeOperatorRepository) FindBestForUpdate(
	_ context.Context,
	_ domain.CategoryID,
	_ time.Time,
) (ports.OperatorCandidate, error) {
	repository.findCalls++
	return repository.candidate, repository.findErr
}

func (repository *fakeOperatorRepository) LockEligibleForUpdate(
	_ context.Context,
	_ domain.UserID,
	_ domain.CategoryID,
	_ time.Time,
) (ports.OperatorCandidate, error) {
	repository.lockCalls++
	return repository.candidate, repository.lockErr
}

func (repository *fakeOperatorRepository) LockActiveForUpdate(
	_ context.Context,
	_ domain.UserID,
) error {
	repository.activeLockCalls++
	return repository.activeErr
}

func (repository *fakeOperatorRepository) IsEligible(
	_ context.Context,
	_ domain.UserID,
	_ domain.CategoryID,
	_ time.Time,
) (bool, error) {
	repository.eligibilityCalls++
	return repository.eligible, repository.eligibleErr
}

func (repository *fakeOperatorRepository) MarkAssigned(
	_ context.Context,
	operatorID domain.UserID,
	assignedAt time.Time,
) error {
	repository.markCalls++
	repository.markedOperatorID = operatorID
	repository.markedAt = assignedAt
	return repository.markErr
}

type assignmentEnvironment struct {
	testEnvironment
	operators *fakeOperatorRepository
}

func newAssignmentEnvironment() assignmentEnvironment {
	environment := newTestEnvironment()
	operators := &fakeOperatorRepository{eligible: true}
	environment.unit.repositories.Operators = operators

	return assignmentEnvironment{
		testEnvironment: environment,
		operators:       operators,
	}
}
