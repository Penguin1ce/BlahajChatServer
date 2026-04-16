package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// R 统一响应信封
type R struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, R{
		Code:    http.StatusOK,
		Message: "success",
		Data:    data,
	})
}

func Err(c *gin.Context, status int, err error) {
	c.JSON(status, R{
		Code:    status,
		Message: err.Error(),
		Data:    nil,
	})
}

func ErrMsg(c *gin.Context, status int, msg string) {
	c.JSON(status, R{
		Code:    status,
		Message: msg,
		Data:    nil,
	})
}
