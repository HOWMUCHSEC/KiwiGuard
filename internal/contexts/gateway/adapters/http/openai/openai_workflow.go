package openai

import (
	"context"
	"net/http"
)

// openAIExchange holds transport-scoped state for one OpenAI-compatible request.
type openAIExchange struct {
	provider providerConfig
	meta     requestMeta
	request  openAIRequest
}

// openAIWorkflowDriver adapts HTTP transport operations to the application exchange workflow.
type openAIWorkflowDriver struct {
	server       *server
	w            http.ResponseWriter
	r            *http.Request
	exchange     openAIExchange
	upstreamBody []byte
}

func (d *openAIWorkflowDriver) PrepareExchange(context.Context) (func(), bool) {
	exchange, release, ok := d.server.prepareOpenAIExchange(d.w, d.r)
	d.exchange = exchange
	return release, ok
}

func (d *openAIWorkflowDriver) ApplyInputPolicy(context.Context) bool {
	return d.server.applyOpenAIInputPolicy(d.w, d.r, &d.exchange)
}

func (d *openAIWorkflowDriver) ProjectUpstreamRequest(context.Context) bool {
	upstreamBody, ok := d.server.projectOpenAIUpstreamRequest(d.w, d.r, d.exchange)
	d.upstreamBody = upstreamBody
	return ok
}

func (d *openAIWorkflowDriver) ForwardExchange(context.Context) {
	d.server.forwardOpenAIExchange(d.w, d.r, d.exchange, d.upstreamBody)
}
