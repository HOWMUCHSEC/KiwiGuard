package httpapi

import appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"

// PolicyStore aliases the control-plane repository port used by the HTTP adapter.
type PolicyStore = appcontrol.Repository

// ActivationNotifier aliases the control-plane activation notifier port used by the HTTP adapter.
type ActivationNotifier = appcontrol.ActivationNotifier

var (
	errGatewayClientNotFound      = appcontrol.ErrGatewayClientNotFound
	errGatewayClientAlreadyExists = appcontrol.ErrGatewayClientAlreadyExists
)
