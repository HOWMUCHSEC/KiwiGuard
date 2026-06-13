package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// listVerdictProviders returns all configured verdict providers.
func (c *policyController) listVerdictProviders(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.ListVerdictProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_verdict_providers_failed", "list verdict providers failed")
		return
	}
	writeJSON(w, http.StatusOK, verdictProviderListResponse{Items: verdictProvidersFromApp(items)})
}

// putVerdictProvider upserts one verdict provider from the request body.
func (c *policyController) putVerdictProvider(w http.ResponseWriter, r *http.Request) {
	var provider verdictProviderDTO
	if !decodeJSON(w, r, &provider) {
		return
	}
	if !normalizePathID(w, chi.URLParam(r, "id"), &provider.ID) {
		return
	}

	appProvider := verdictProviderToApp(provider)
	if err := c.service.PutVerdictProvider(r.Context(), appProvider); err != nil {
		writeError(w, verdictProviderStatus(err), verdictProviderErrorCode(err), err.Error())
		return
	}
	appProvider.Mode = "inline"
	writeJSON(w, http.StatusOK, verdictProviderFromApp(appProvider))
}
