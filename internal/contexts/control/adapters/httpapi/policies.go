package httpapi

import (
	"net/http"

	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

// policyController binds HTTP handlers to control-plane application services.
type policyController struct {
	service *appcontrol.Service
}

// newPolicyController builds a controller backed by the control-plane application service.
func newPolicyController(store PolicyStore, notifier ActivationNotifier) *policyController {
	return &policyController{
		service: appcontrol.NewService(appcontrol.ServiceOptions{
			Repository: store,
			Notifier:   notifier,
		}),
	}
}

// configStatus serves the active bundle set and compiled snapshot hash through the control API.
func (c *policyController) configStatus(w http.ResponseWriter, r *http.Request) {
	status, err := c.service.ConfigStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_status_failed", "config status failed")
		return
	}
	writeJSON(w, http.StatusOK, configStatusFromApp(status))
}
