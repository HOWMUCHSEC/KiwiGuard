package httpapi

import (
	"errors"
	"net/http"
	"regexp"

	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

// regexTest runs a regex against sample text for detector authoring workflows.
func (c *policyController) regexTest(w http.ResponseWriter, r *http.Request) {
	var request regexTestRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	re, err := regexp.Compile(request.Pattern)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_regex", err.Error())
		return
	}

	indexes := re.FindAllStringIndex(request.Text, -1)
	matches := make([]regexMatchDTO, 0, len(indexes))
	for _, index := range indexes {
		matches = append(matches, regexMatchDTO{
			Start: index[0],
			End:   index[1],
			Text:  request.Text[index[0]:index[1]],
		})
	}

	writeJSON(w, http.StatusOK, regexTestResponse{Matches: matches})
}

// policyDryRun evaluates one bundle against supplied text without persisting it.
func (c *policyController) policyDryRun(w http.ResponseWriter, r *http.Request) {
	var request policyDryRunRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	response, err := c.service.PolicyDryRun(policyDryRunRequestToApp(request))
	if err != nil {
		if errors.Is(err, appcontrol.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, policyDryRunErrorCode(request), err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "policy_dry_run_failed", "policy dry run failed")
		return
	}
	writeJSON(w, http.StatusOK, policyDryRunResponse{Decision: toDecisionDTO(response.Decision)})
}

// policyDryRunErrorCode maps invalid dry-run input to a stable API error code.
func policyDryRunErrorCode(request policyDryRunRequest) string {
	if request.Direction != "input" && request.Direction != "output" {
		return "invalid_direction"
	}
	return "invalid_policy_bundle"
}
