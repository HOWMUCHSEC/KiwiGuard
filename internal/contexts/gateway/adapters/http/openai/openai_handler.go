package openai

import (
	"net/http"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// handleOpenAI delegates one OpenAI-compatible request to the gateway application workflow.
func (s *server) handleOpenAI(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	_ = (appgateway.ExchangeWorkflowUseCase{}).Run(r.Context(), &openAIWorkflowDriver{
		server: s,
		w:      w,
		r:      r,
	})
}
