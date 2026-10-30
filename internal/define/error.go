package define

// BusinessError exposes a controlled message while retaining an internal cause.
type BusinessError struct {
	Code      string
	Message   string
	Operation string `json:"-"`
	Cause     error  `json:"-"`
}

func (e *BusinessError) Error() string { return e.Message }
func (e *BusinessError) Unwrap() error { return e.Cause }
func ClientError(message string) error {
	return &BusinessError{Code: CodeClientError, Message: message}
}
func ServerError(message, operation string, cause error) error {
	return &BusinessError{Code: CodeServerError, Message: message, Operation: operation, Cause: cause}
}
