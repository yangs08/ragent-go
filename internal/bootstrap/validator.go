package bootstrap

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// 在请求处理前注册一次，HTTP 绑定和 CLI 显式校验共用 Gin 的 validator。
// 只注册字节长度、非空白等通用规则；用户名等业务规则由对应 Service 判断。
func init() {
	v := binding.Validator.Engine().(*validator.Validate)
	// 内置 min/max 对字符串统计字符数，bcrypt 需要按字节限制。
	_ = v.RegisterValidation("minbytes", func(fl validator.FieldLevel) bool {
		n, err := strconv.Atoi(fl.Param())
		return err == nil && len(fl.Field().String()) >= n
	})
	_ = v.RegisterValidation("maxbytes", func(fl validator.FieldLevel) bool {
		n, err := strconv.Atoi(fl.Param())
		return err == nil && len(fl.Field().String()) <= n
	})
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	})
}
