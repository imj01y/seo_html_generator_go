// Package core provides panic recovery middleware
package core

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Recovery returns a gin middleware that recovers from panics
// It logs the panic stack trace and returns a unified error response
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := getRequestIDFromContext(c)
				stack := getStackTrace(3) // Skip recover, panic, and this function

				// Log the panic with full details
				log.Error().
					Str("request_id", requestID).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Str("client_ip", c.ClientIP()).
					Str("user_agent", c.Request.UserAgent()).
					Interface("error", err).
					Str("stack", stack).
					Msg("Panic recovered")

				// Send error response
				c.AbortWithStatusJSON(http.StatusInternalServerError, Response{
					Code:      int(ErrInternalServer),
					Message:   GetErrorMessage(ErrInternalServer),
					Timestamp: time.Now().Unix(),
					RequestID: requestID,
				})
			}
		}()

		c.Next()
	}
}

// getStackTrace returns a formatted stack trace string
func getStackTrace(skip int) string {
	var builder strings.Builder

	// Get up to 32 stack frames
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip+1, pcs)
	pcs = pcs[:n]

	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()

		// Skip runtime internal frames
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}

		builder.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))

		if !more {
			break
		}
	}

	return builder.String()
}

// getRequestIDFromContext extracts request ID from gin context
func getRequestIDFromContext(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if idStr, ok := id.(string); ok {
			return idStr
		}
	}
	return ""
}
