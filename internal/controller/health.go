package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthController struct{}

func (HealthController) Check(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	_ = json.NewEncoder(c.Writer).Encode(map[string]string{"status": "ok"})
}
func (HealthController) Head(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusOK)
}
