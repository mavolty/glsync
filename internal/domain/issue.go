package domain

type WorkflowState string

const (
	StateInProgress WorkflowState = "in_progress"
	StateCodeReview WorkflowState = "code_review"
	StateRFQA       WorkflowState = "rfqa"
	StateDone       WorkflowState = "done"
)
