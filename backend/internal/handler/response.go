// Package handler contains the HTTP layer: thin functions that parse requests,
// call services, and format responses according to the conventions in CLAUDE.md.
package handler

import "github.com/gin-gonic/gin"

// RespondData writes a successful response body of the form {"data": ...}.
func RespondData(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// RespondError writes the standard error envelope.
func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": code, "message": message})
}

// RespondErrorDetails is RespondError with an extra "details" object.
func RespondErrorDetails(c *gin.Context, status int, code, message string, details any) {
	c.JSON(status, gin.H{"error": code, "message": message, "details": details})
}
