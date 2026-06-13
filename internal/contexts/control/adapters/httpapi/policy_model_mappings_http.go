package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// listModelMappings returns all configured model mappings.
func (c *policyController) listModelMappings(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.ListModelMappings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_model_mappings_failed", "list model mappings failed")
		return
	}
	writeJSON(w, http.StatusOK, modelMappingListResponse{Items: modelMappingsFromApp(items)})
}

// putModelMapping upserts one model mapping from the request body.
func (c *policyController) putModelMapping(w http.ResponseWriter, r *http.Request) {
	var mapping modelMappingDTO
	if !decodeJSON(w, r, &mapping) {
		return
	}
	if !normalizePathID(w, chi.URLParam(r, "id"), &mapping.ID) {
		return
	}

	if err := c.service.PutModelMapping(r.Context(), modelMappingToApp(mapping)); err != nil {
		writeError(w, http.StatusInternalServerError, "put_model_mapping_failed", "put model mapping failed")
		return
	}
	writeJSON(w, http.StatusOK, mapping)
}
