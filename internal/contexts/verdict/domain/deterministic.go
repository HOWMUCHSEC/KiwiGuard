package verdict

import (
	"context"
	"time"
)

type deterministicProvider struct {
	result Result
}

// NewDeterministicProvider returns a provider that always returns result.
func NewDeterministicProvider(result Result) Provider {
	return deterministicProvider{result: result}
}

func (p deterministicProvider) Evaluate(ctx context.Context, request Request) (Result, error) {
	start := time.Now()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	result := p.result
	result.Categories = append([]string(nil), p.result.Categories...)
	result.MatchedSpans = append([]MatchedSpan(nil), p.result.MatchedSpans...)
	result.Latency = time.Since(start)
	if result.Latency <= 0 {
		result.Latency = time.Nanosecond
	}
	return result, nil
}
