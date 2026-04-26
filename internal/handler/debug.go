package handler

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	//go:embed assets/ws_tester.html
	wsTesterHTML string
)

func WSTesterPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(wsTesterHTML))
}
