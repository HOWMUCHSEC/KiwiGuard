package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// listGatewayClients returns all configured gateway clients.
func (c *policyController) listGatewayClients(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.ListGatewayClients(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_gateway_clients_failed", "list gateway clients failed")
		return
	}
	writeJSON(w, http.StatusOK, gatewayClientListResponse{Items: gatewayClientsFromApp(items)})
}

// createGatewayClient serves one-time key creation and persists the resulting gateway client record.
func (c *policyController) createGatewayClient(w http.ResponseWriter, r *http.Request) {
	var request createGatewayClientRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	response, err := c.service.CreateGatewayClient(r.Context(), createGatewayClientRequestToApp(request))
	if err != nil {
		writeGatewayClientStoreError(w, err, gatewayClientErrorCode(err, "create_gateway_client_failed"), gatewayClientErrorMessage(err, "create gateway client failed"))
		return
	}
	writeJSON(w, http.StatusCreated, createGatewayClientResponseFromApp(response))
}

// patchGatewayClient updates mutable gateway client metadata.
func (c *policyController) patchGatewayClient(w http.ResponseWriter, r *http.Request) {
	var client gatewayClientDTO
	if !decodeJSON(w, r, &client) {
		return
	}
	if !normalizePathID(w, strings.TrimSpace(chi.URLParam(r, "client_id")), &client.ID) {
		return
	}
	updated, err := c.service.PatchGatewayClient(r.Context(), gatewayClientToApp(client))
	if err != nil {
		writeGatewayClientStoreError(w, err, gatewayClientErrorCode(err, "patch_gateway_client_failed"), gatewayClientErrorMessage(err, "patch gateway client failed"))
		return
	}
	writeJSON(w, http.StatusOK, gatewayClientFromApp(updated))
}

// revokeGatewayClient revokes an existing gateway client key.
func (c *policyController) revokeGatewayClient(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(chi.URLParam(r, "client_id"))
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "invalid_gateway_client", "client_id is required")
		return
	}

	client, err := c.service.RevokeGatewayClient(r.Context(), clientID)
	if err != nil {
		writeGatewayClientStoreError(w, err, "revoke_gateway_client_failed", "revoke gateway client failed")
		return
	}
	writeJSON(w, http.StatusOK, gatewayClientFromApp(client))
}
