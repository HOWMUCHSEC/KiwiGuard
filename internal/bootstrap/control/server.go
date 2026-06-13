// Package control assembles the control-plane HTTP server.
package control

import (
	"net/http"

	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
)

// BuildHandler assembles the control-plane HTTP handler from transport server options.
func BuildHandler(options controlhttp.ServerOptions) http.Handler {
	return controlhttp.NewServer(options)
}
