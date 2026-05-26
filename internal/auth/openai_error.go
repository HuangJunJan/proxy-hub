package auth

import "github.com/gin-gonic/gin"

type OpenAIErrorBody struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func AbortOpenAIError(c *gin.Context, status int, message, typ, code string) {
	c.AbortWithStatusJSON(status, OpenAIErrorBody{
		Error: OpenAIError{
			Message: message,
			Type:    typ,
			Code:    code,
		},
	})
}
