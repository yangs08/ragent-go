package define

const (
	CodeSuccess     = "0"       // 请求处理成功。
	CodeClientError = "A000001" // 参数、身份等可预期的客户端错误。
	CodeServerError = "B000001" // 数据库、模型调用等内部处理错误。
)

// Result 定义普通业务 HTTP 接口的统一响应结构，由 common 输出函数填充。
// 健康检查及 SSE 使用各自协议，不套用此结构。
type Result struct {
	Code      string `json:"code"`              // 业务结果码，不是 HTTP 状态码。
	Message   string `json:"message,omitempty"` // 可展示给用户的提示；为空时省略。
	Data      any    `json:"data"`              // 业务数据；无数据时返回 null。
	RequestID string `json:"requestId"`         // 对应 X-Request-ID，便于关联服务端日志。
}
