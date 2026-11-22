package define

// BusinessError 在业务层与输出层之间传递错误分类、安全提示和内部原因。
// OutputError 根据 Code/Message 生成响应；日志与 errors.Is/As 可追踪 Cause。
type BusinessError struct {
	// 业务错误码，区分客户端错误 A000001 与系统错误 B000001。
	Code string
	// 可直接展示给用户的提示，不应包含 SQL、凭据或模型供应商的原始响应。
	Message string
	// 内部操作名称，用于定位失败步骤，不进入 HTTP 响应。
	Operation string `json:"-"`
	// 原始错误，保留错误链供诊断；不直接输出给用户或记录完整文本。
	Cause error `json:"-"`
}

// Error 仅返回安全提示，避免调用方打印 error 时泄露原始原因。
func (e *BusinessError) Error() string { return e.Message }

// Unwrap 让 errors.Is/As 可以识别内部的超时、取消及数据库等错误。
func (e *BusinessError) Unwrap() error { return e.Cause }

// ClientError 表示参数不合法、未登录等可预期失败，由输出层返回客户端错误码。
func ClientError(message string) error {
	return &BusinessError{Code: CodeClientError, Message: message}
}

// ServerError 表示数据库、模型服务等内部失败，分别保存安全提示和诊断信息。
func ServerError(message, operation string, cause error) error {
	return &BusinessError{Code: CodeServerError, Message: message, Operation: operation, Cause: cause}
}
