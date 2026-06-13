package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

// normalizePathID enforces that path and body identifiers refer to the same resource.
func normalizePathID(w http.ResponseWriter, pathID string, bodyID *string) bool {
	if *bodyID != "" && *bodyID != pathID {
		writeError(w, http.StatusBadRequest, "id_mismatch", "path id and body id must match")
		return false
	}
	*bodyID = pathID
	return true
}

// decodeJSON decodes one request body into target and writes a bad-request error on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer func() {
		_ = r.Body.Close()
	}()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return false
	}
	return true
}

// writeJSON writes a JSON response with the provided status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError writes the standard control-plane JSON error envelope.
func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeGatewayClientStoreError(w http.ResponseWriter, err error, fallbackCode string, fallbackMessage string) {
	switch {
	case errors.Is(err, appcontrol.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, fallbackCode, err.Error())
	case errors.Is(err, errGatewayClientNotFound):
		writeError(w, http.StatusNotFound, "gateway_client_not_found", "gateway client not found")
	case errors.Is(err, errGatewayClientAlreadyExists):
		writeError(w, http.StatusConflict, "gateway_client_exists", "gateway client already exists")
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage)
	}
}

func gatewayClientErrorMessage(err error, fallback string) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return err.Error()
	}
	return fallback
}

func gatewayClientErrorCode(err error, fallback string) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return "invalid_gateway_client"
	}
	return fallback
}

func policyBundleStatus(err error) int {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func policyBundleErrorCode(err error, fallback string) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return "invalid_policy_bundle"
	}
	return fallback
}

func activationStatus(err error) int {
	if errors.Is(err, errGatewayClientNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func verdictProviderStatus(err error) int {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func verdictProviderErrorCode(err error) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return "unsupported_verdict_provider_mode"
	}
	return "put_verdict_provider_failed"
}

func routeLimitStatus(err error) int {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func routeLimitErrorCode(err error, fallback string) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return "invalid_route_limit"
	}
	return fallback
}

func clientRouteLimitErrorCode(err error, fallback string) string {
	if errors.Is(err, appcontrol.ErrInvalidInput) {
		return "invalid_client_route_limit"
	}
	return fallback
}
