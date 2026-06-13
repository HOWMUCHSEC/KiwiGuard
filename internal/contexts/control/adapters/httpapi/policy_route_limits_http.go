package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// listRouteLimits serves the default route-limit policies currently visible through the control API.
func (c *policyController) listRouteLimits(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.ListRouteLimits(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_route_limits_failed", "list route limits failed")
		return
	}
	writeJSON(w, http.StatusOK, routeLimitListResponse{Items: routeLimitsFromApp(items)})
}

// putRouteLimit upserts the default limit policy for one route.
func (c *policyController) putRouteLimit(w http.ResponseWriter, r *http.Request) {
	var limit routeLimitDTO
	if !decodeJSON(w, r, &limit) {
		return
	}
	if !normalizePathID(w, strings.TrimSpace(chi.URLParam(r, "route_key")), &limit.RouteKey) {
		return
	}
	saved, err := c.service.PutRouteLimit(r.Context(), routeLimitToApp(limit))
	if err != nil {
		writeError(w, routeLimitStatus(err), routeLimitErrorCode(err, "put_route_limit_failed"), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routeLimitFromApp(saved))
}

// listClientRouteLimits returns all route limit overrides for one client.
func (c *policyController) listClientRouteLimits(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(chi.URLParam(r, "client_id"))
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "invalid_client_route_limit", "client_id is required")
		return
	}

	items, err := c.service.ListClientRouteLimits(r.Context(), clientID)
	if err != nil {
		writeGatewayClientStoreError(w, err, "list_client_route_limits_failed", "list client route limits failed")
		return
	}
	writeJSON(w, http.StatusOK, clientRouteLimitListResponse{Items: clientRouteLimitsFromApp(items)})
}

// putClientRouteLimit upserts one client-specific route limit override.
func (c *policyController) putClientRouteLimit(w http.ResponseWriter, r *http.Request) {
	var limit clientRouteLimitDTO
	if !decodeJSON(w, r, &limit) {
		return
	}
	if !normalizePathID(w, strings.TrimSpace(chi.URLParam(r, "client_id")), &limit.ClientID) {
		return
	}
	if !normalizePathID(w, strings.TrimSpace(chi.URLParam(r, "route_key")), &limit.RouteKey) {
		return
	}
	saved, err := c.service.PutClientRouteLimit(r.Context(), clientRouteLimitToApp(limit))
	if err != nil {
		writeGatewayClientStoreError(w, err, clientRouteLimitErrorCode(err, "put_client_route_limit_failed"), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, clientRouteLimitFromApp(saved))
}

// deleteClientRouteLimit removes one client-specific route limit override.
func (c *policyController) deleteClientRouteLimit(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimSpace(chi.URLParam(r, "client_id"))
	routeKey := strings.TrimSpace(chi.URLParam(r, "route_key"))
	if clientID == "" {
		writeError(w, http.StatusBadRequest, "invalid_client_route_limit", "client_id is required")
		return
	}
	if routeKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_client_route_limit", "route key is required")
		return
	}

	if err := c.service.DeleteClientRouteLimit(r.Context(), clientID, routeKey); err != nil {
		writeGatewayClientStoreError(w, err, clientRouteLimitErrorCode(err, "delete_client_route_limit_failed"), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
