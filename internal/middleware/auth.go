package middleware

import (
	"github.com/gin-gonic/gin"

	"ragent-go/internal/common"
	"ragent-go/internal/define"
	"ragent-go/internal/service"
)

// RequireLogin verifies a raw Authorization token, then binds the trusted user.
// Future chat and conversation routes must use this same middleware.
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 {
			common.OutputError(c, define.ClientError("未登录或登录已过期"))
			return
		}
		user, err := service.Auth.Authenticate(c.Request.Context(), values[0])
		if err != nil {
			common.OutputError(c, err)
			return
		}
		c.Request = c.Request.WithContext(common.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
