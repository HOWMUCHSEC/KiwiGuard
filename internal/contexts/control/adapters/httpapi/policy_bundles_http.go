package httpapi

import "net/http"

// listPolicyBundles returns all configured policy bundles.
func (c *policyController) listPolicyBundles(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.ListPolicyBundles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_policy_bundles_failed", "list policy bundles failed")
		return
	}
	writeJSON(w, http.StatusOK, policyBundleListResponse{Items: policyBundlesFromApp(items)})
}

// createPolicyBundle validates and persists one policy bundle from the request body.
func (c *policyController) createPolicyBundle(w http.ResponseWriter, r *http.Request) {
	var bundle policyBundleDTO
	if !decodeJSON(w, r, &bundle) {
		return
	}
	appBundle := policyBundleToApp(bundle)
	if err := c.service.CreatePolicyBundle(r.Context(), appBundle); err != nil {
		writeError(w, policyBundleStatus(err), policyBundleErrorCode(err, "create_policy_bundle_failed"), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, bundle)
}

// validatePolicyBundle validates one policy bundle without persisting it.
func (c *policyController) validatePolicyBundle(w http.ResponseWriter, r *http.Request) {
	var bundle policyBundleDTO
	if !decodeJSON(w, r, &bundle) {
		return
	}
	appBundle := policyBundleToApp(bundle)
	hash, err := c.service.ValidatePolicyBundle(appBundle)
	if err != nil {
		writeJSON(w, http.StatusOK, policyValidationResponse{Valid: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, policyValidationResponse{Valid: true, Hash: hash})
}

// activatePolicyBundles promotes the requested bundle set to the active revision.
func (c *policyController) activatePolicyBundles(w http.ResponseWriter, r *http.Request) {
	var request policyActivationRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	response, err := c.service.ActivatePolicyBundles(r.Context(), policyActivationRequestToApp(request))
	if err != nil {
		writeError(w, activationStatus(err), "activate_policy_bundles_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policyActivationResponseFromApp(response))
}
