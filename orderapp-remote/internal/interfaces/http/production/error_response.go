package production

type ErrorResponse struct {
	Error        string `json:"error"`
	Code         string `json:"code,omitempty"`
	NotCommitted bool   `json:"not_committed,omitempty"`
}
