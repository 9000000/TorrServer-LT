package api

import (
	"github.com/gin-gonic/gin"
)

// abortWithJSONError aborts the request with {"error": "..."} as the body.
// gin's AbortWithError only sets the status and records the error, leaving the
// response empty, which breaks clients that parse every reply as JSON.
func abortWithJSONError(c *gin.Context, code int, err error) {
	c.AbortWithStatusJSON(code, gin.H{"error": err.Error()})
}
