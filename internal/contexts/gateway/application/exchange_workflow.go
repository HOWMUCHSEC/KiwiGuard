package application

import "context"

// ExchangeWorkflowUseCase owns the top-level gateway exchange lifecycle order.
type ExchangeWorkflowUseCase struct{}

// ExchangeWorkflowDriver supplies transport-specific operations for one gateway exchange.
type ExchangeWorkflowDriver interface {
	PrepareExchange(context.Context) (func(), bool)
	ApplyInputPolicy(context.Context) bool
	ProjectUpstreamRequest(context.Context) bool
	ForwardExchange(context.Context)
}

// Run executes one gateway exchange through preparation, input policy, projection, and forwarding.
func (ExchangeWorkflowUseCase) Run(ctx context.Context, driver ExchangeWorkflowDriver) bool {
	if driver == nil {
		return false
	}

	release, ok := driver.PrepareExchange(ctx)
	if !ok {
		return false
	}
	if release != nil {
		defer release()
	}

	if !driver.ApplyInputPolicy(ctx) {
		return false
	}
	if !driver.ProjectUpstreamRequest(ctx) {
		return false
	}
	driver.ForwardExchange(ctx)
	return true
}
