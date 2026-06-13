package application

import (
	"context"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
)

// ShadowEvaluationObserver receives completed async shadow verdict evaluations.
type ShadowEvaluationObserver interface {
	ObserveShadowEvaluation(context.Context, TrafficEvaluationResult)
}

// TrafficEvaluationUseCase owns verdict and policy orchestration for gateway traffic.
type TrafficEvaluationUseCase struct {
	Snapshot       PolicySnapshot
	VerdictTimeout time.Duration
	ShadowObserver ShadowEvaluationObserver
}

// Evaluate evaluates one traffic payload against the configured verdict and policy lifecycle.
func (u TrafficEvaluationUseCase) Evaluate(ctx context.Context, input TrafficEvaluationInput) TrafficEvaluationResult {
	result := TrafficEvaluationResult{Direction: input.Direction, Text: input.Text}
	if input.Execution == routing.ExecutionAsyncShadow {
		u.startAsyncShadow(ctx, input)
		result.Decision, result.DetectorLatency = u.evaluatePolicyTimed(input, policy.ModelSignal{})
		return result
	}

	modelSignal := policy.ModelSignal{}
	if input.VerdictProvider != nil {
		verdictStart := time.Now()
		verdictResult, err := u.evaluateVerdict(ctx, input)
		result.VerdictLatency = positiveDurationSince(verdictStart)
		result.Verdict = verdictResult
		modelSignal = ModelSignalFromVerdict(verdictResult, err, input.FallbackAction)
	}
	result.Decision, result.DetectorLatency = u.evaluatePolicyTimed(input, modelSignal)
	return result
}

// EvaluatePolicyOnly evaluates traffic against local policy without invoking verdict providers.
func (u TrafficEvaluationUseCase) EvaluatePolicyOnly(input TrafficEvaluationInput) TrafficEvaluationResult {
	decision, detectorLatency := u.evaluatePolicyTimed(input, policy.ModelSignal{})
	return TrafficEvaluationResult{
		Direction:       input.Direction,
		Text:            input.Text,
		Decision:        decision,
		DetectorLatency: detectorLatency,
	}
}

func (u TrafficEvaluationUseCase) startAsyncShadow(ctx context.Context, input TrafficEvaluationInput) {
	if input.VerdictProvider == nil || u.ShadowObserver == nil {
		return
	}
	go func() {
		shadowCtx := context.WithoutCancel(ctx)
		verdictStart := time.Now()
		verdictResult, err := u.evaluateVerdictBounded(shadowCtx, input)
		modelSignal := ModelSignalFromVerdict(verdictResult, err, input.FallbackAction)
		decision, detectorLatency := u.evaluatePolicyTimed(input, modelSignal)
		u.ShadowObserver.ObserveShadowEvaluation(shadowCtx, TrafficEvaluationResult{
			Direction:       input.Direction,
			Text:            input.Text,
			Verdict:         verdictResult,
			Decision:        decision,
			DetectorLatency: detectorLatency,
			VerdictLatency:  positiveDurationSince(verdictStart),
		})
	}()
}

func (u TrafficEvaluationUseCase) evaluateVerdict(ctx context.Context, input TrafficEvaluationInput) (verdict.Result, error) {
	if u.VerdictTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, u.VerdictTimeout)
		defer cancel()
	}
	return input.VerdictProvider.Evaluate(ctx, verdictRequest(input))
}

func (u TrafficEvaluationUseCase) evaluateVerdictBounded(ctx context.Context, input TrafficEvaluationInput) (verdict.Result, error) {
	if u.VerdictTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, u.VerdictTimeout)
		defer cancel()
	}
	return evaluateVerdictBounded(ctx, input.VerdictProvider, verdictRequest(input))
}

func (u TrafficEvaluationUseCase) evaluatePolicyTimed(input TrafficEvaluationInput, signal policy.ModelSignal) (policy.Decision, time.Duration) {
	start := time.Now()
	decision := PolicyEvaluator{Snapshot: u.Snapshot}.Evaluate(PolicyEvaluationInput{
		RouteKey:       input.RouteKey,
		ProviderKey:    input.ProviderKey,
		Model:          input.Model,
		Direction:      input.Direction,
		Text:           input.Text,
		ModelSignal:    signal,
		FallbackAction: PolicyAction(input.FallbackAction),
	})
	return decision, positiveDurationSince(start)
}

func evaluateVerdictBounded(ctx context.Context, provider verdict.Provider, request verdict.Request) (verdict.Result, error) {
	resultCh := make(chan verdictOutcome, 1)
	go func() {
		result, err := provider.Evaluate(ctx, request)
		resultCh <- verdictOutcome{result: result, err: err}
	}()

	select {
	case outcome := <-resultCh:
		return outcome.result, outcome.err
	case <-ctx.Done():
		return verdict.Result{}, ctx.Err()
	}
}

func verdictRequest(input TrafficEvaluationInput) verdict.Request {
	return verdict.Request{
		RequestID:     input.RequestID,
		CorrelationID: input.CorrelationID,
		RouteKey:      input.RouteKey,
		ProviderKey:   input.ProviderKey,
		Model:         input.Model,
		Direction:     verdict.Direction(input.Direction),
		Text:          input.Text,
	}
}

type verdictOutcome struct {
	result verdict.Result
	err    error
}

func positiveDurationSince(start time.Time) time.Duration {
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return time.Nanosecond
	}
	return elapsed
}
