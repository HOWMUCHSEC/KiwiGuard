package openai

import (
	"encoding/json"
	"net/http"
)

// openAIError is the OpenAI-compatible error envelope returned by the gateway.
type openAIError struct {
	Error openAIErrorBody `json:"error"`
}

// openAIErrorBody is the inner OpenAI-compatible error payload.
type openAIErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// writeOpenAIError writes an OpenAI-compatible JSON error response.
func writeOpenAIError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openAIError{
		Error: openAIErrorBody{
			Message: message,
			Type:    "invalid_request_error",
			Code:    code,
		},
	})
}
