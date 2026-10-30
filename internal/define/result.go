package define

const (
	CodeSuccess     = "0"
	CodeClientError = "A000001"
	CodeServerError = "B000001"
)

// Result is the common envelope for all business HTTP responses.
// Health and SSE frames retain their own protocols.
type Result struct {
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
}
