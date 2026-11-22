package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

var Health = new(healthController)

type healthController struct{}

func (*healthController) Check(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	_ = json.NewEncoder(c.Writer).Encode(map[string]string{"status": "ok"})
}
func (*healthController) Head(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusOK)
}
