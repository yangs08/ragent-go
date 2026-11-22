package common

import (
	"context"
	"errors"
	"fmt"

	"ragent-go/internal/define"
)

// FailureAttrs 将错误转换为 slog 使用的键值对，只提取允许记录的诊断字段。
// 不记录 err.Error()：原始错误可能包含 SQL、凭据或供应商响应正文。
func FailureAttrs(operation string, err error) []any {
	// BusinessError 对外提供安全提示；日志分类需要查看它包装的内部原因。
	var business *define.BusinessError
	if errors.As(err, &business) && business.Cause != nil {
		err = business.Cause
	}
	attrs := []any{"operation", operation, "error_type", fmt.Sprintf("%T", err)}
	// 通过能力接口提取 SQLSTATE，无需依赖具体数据库驱动；errors.As 可穿透错误包装。
	// SQLSTATE 是数据库错误类别码，不是 SQL 语句或查询参数。
	var sql interface{ SQLState() string }
	if errors.As(err, &sql) {
		attrs = append(attrs, "sqlstate", sql.SQLState())
	}
	// 用 errors.Is 识别包装后的取消/超时，避免依赖可能变化的错误文本。
	if errors.Is(err, context.Canceled) {
		attrs = append(attrs, "reason", "canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		attrs = append(attrs, "reason", "deadline_exceeded")
	}
	return attrs
}
