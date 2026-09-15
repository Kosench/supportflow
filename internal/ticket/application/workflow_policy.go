package application

import "github.com/Kosench/supportflow/internal/ticket/domain"

type WorkflowAction string

const (
	WorkflowStartProgress       WorkflowAction = "start_progress"
	WorkflowWaitForCustomer     WorkflowAction = "wait_for_customer"
	WorkflowResumeProgress      WorkflowAction = "resume_progress"
	WorkflowRecordFirstResponse WorkflowAction = "record_first_response"
	WorkflowResolve             WorkflowAction = "resolve"
	WorkflowClose               WorkflowAction = "close"
	WorkflowReopen              WorkflowAction = "reopen"
	WorkflowUnassign            WorkflowAction = "unassign"
)

func (action WorkflowAction) Valid() bool {
	switch action {
	case WorkflowStartProgress,
		WorkflowWaitForCustomer,
		WorkflowResumeProgress,
		WorkflowRecordFirstResponse,
		WorkflowResolve,
		WorkflowClose,
		WorkflowReopen,
		WorkflowUnassign:
		return true
	default:
		return false
	}
}

func authorizeWorkFlow(actor Actor, action WorkflowAction, snapshot domain.TicketSnapshot) error {
	if !action.Valid() {
		return deny(actor, "ticket.workflow.unknow")
	}

	if actor.Role == RoleManager || actor.Role == RoleAdmin {
		return nil
	}

	allowed := false
	switch action {
	case WorkflowStartProgress,
		WorkflowWaitForCustomer,
		WorkflowRecordFirstResponse,
		WorkflowResolve:
		allowed = actor.Role == RoleOperator &&
			isAssignee(actor, snapshot)

	case WorkflowResumeProgress:
		allowed = (actor.Role == RoleUser &&
			isRequester(actor, snapshot)) ||
			(actor.Role == RoleOperator &&
				isAssignee(actor, snapshot))

	case WorkflowClose, WorkflowReopen:
		allowed = actor.Role == RoleUser &&
			isRequester(actor, snapshot)

	case WorkflowUnassign:
		allowed = false
	}

	if !allowed {
		return deny(actor, "ticket.workflow."+string(action))
	}

	return nil
}

func isRequester(actor Actor, snapshot domain.TicketSnapshot) bool {
	return snapshot.RequesterID == actor.UserID
}

func isAssignee(actor Actor, snapshot domain.TicketSnapshot) bool {
	return snapshot.AssigneeID != nil && *snapshot.AssigneeID == actor.UserID
}
