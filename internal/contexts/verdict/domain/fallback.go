package verdict

import (
	"context"
	"time"
)

// FallbackOptions configures fallback behavior for provider errors.
type FallbackOptions struct {
	ProviderName string
	Timeout      time.Duration
	Action       Action
}

type fallbackProvider struct {
	provider Provider
	opts     FallbackOptions
}

// WithFallback wraps provider and returns fallback results for timeouts or errors.
func WithFallback(provider Provider, opts FallbackOptions) Provider {
	return fallbackProvider{
		provider: provider,
		opts:     opts,
	}
}

func (p fallbackProvider) Evaluate(ctx context.Context, request Request) (Result, error) {
	start := time.Now()
	parentCtx := ctx
	if p.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.opts.Timeout)
		defer cancel()
	}

	result, err := p.provider.Evaluate(ctx, request)
	if err == nil {
		if result.Latency <= 0 {
			result.Latency = time.Since(start)
			if result.Latency <= 0 {
				result.Latency = time.Nanosecond
			}
		}
		return result, nil
	}
	if parentErr := parentCtx.Err(); parentErr != nil {
		return Result{}, parentErr
	}

	action := p.opts.Action
	if action == "" {
		action = ActionShadowLog
	}

	return Result{
		RiskLevel:       RiskUnknown,
		Categories:      []string{},
		SuggestedAction: action,
		Latency:         positiveDurationSince(start),
		ProviderName:    p.opts.ProviderName,
		FallbackUsed:    true,
		Error:           err.Error(),
	}, nil
}

func positiveDurationSince(start time.Time) time.Duration {
	latency := time.Since(start)
	if latency <= 0 {
		return time.Nanosecond
	}
	return latency
}
