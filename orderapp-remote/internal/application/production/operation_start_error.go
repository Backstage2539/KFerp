package production

// OperationStartNotCommittedError identifies a rejection before a commit was
// attempted. A commit error itself must remain ambiguous for safe replay.
type OperationStartNotCommittedError struct{ Cause error }

func (e *OperationStartNotCommittedError) Error() string { return e.Cause.Error() }
func (e *OperationStartNotCommittedError) Unwrap() error { return e.Cause }
