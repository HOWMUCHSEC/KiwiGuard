package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
)

func TestTrafficEvaluationUseCaseAppliesInlineVerdictSignal(t *testing.T) {
	useCase := TrafficEvaluationUseCase{
		Snapshot: fakePolicySnapshot{decision: policy.Decision{Action: policy.ActionRedact}},
	}
	provider := verdict.ProviderFunc(func(_ context.Context, request verdict.Request) (verdict.Result, error) {
		if request.RouteKey != "chat" || request.Model != "safe-model" || request.Direction != verdict.DirectionInput {
			t.Fatalf("verdict request = %+v, want route/model/input fields", request)
		}
		return verdict.Result{
			SuggestedAction: verdict.ActionBlock,
			RiskLevel:       verdict.RiskHigh,
			Confidence:      0.92,
		}, nil
	})

	result := useCase.Evaluate(context.Background(), TrafficEvaluationInput{
		RequestID:       "req-1",
		CorrelationID:   "corr-1",
		RouteKey:        "chat",
		ProviderKey:     "openai",
		Model:           "safe-model",
		Direction:       detection.DirectionInput,
		Text:            "hello",
		Execution:       routing.ExecutionInline,
		FallbackAction:  routing.ActionBlock,
		VerdictProvider: provider,
	})

	if result.Direction != detection.DirectionInput || result.Text != "hello" {
		t.Fatalf("result traffic identity = %+v, want input text", result)
	}
	if result.Verdict.SuggestedAction != verdict.ActionBlock {
		t.Fatalf("verdict action = %q, want block", result.Verdict.SuggestedAction)
	}
	if result.Decision.Action != policy.ActionRedact {
		t.Fatalf("decision action = %q, want snapshot decision", result.Decision.Action)
	}
	if result.DetectorLatency <= 0 {
		t.Fatalf("detector latency = %s, want positive duration", result.DetectorLatency)
	}
}

func TestTrafficEvaluationUseCaseRunsAsyncShadowOutOfBand(t *testing.T) {
	shadowResults := make(chan TrafficEvaluationResult, 1)
	useCase := TrafficEvaluationUseCase{
		Snapshot:       fakePolicySnapshot{decision: policy.Decision{Action: policy.ActionAllow}},
		VerdictTimeout: time.Second,
		ShadowObserver: shadowObserverFunc(func(_ context.Context, result TrafficEvaluationResult) {
			shadowResults <- result
		}),
	}
	provider := verdict.ProviderFunc(func(_ context.Context, _ verdict.Request) (verdict.Result, error) {
		return verdict.Result{SuggestedAction: verdict.ActionBlock, RiskLevel: verdict.RiskCritical}, nil
	})

	result := useCase.Evaluate(context.Background(), TrafficEvaluationInput{
		RequestID:       "req-1",
		CorrelationID:   "corr-1",
		RouteKey:        "chat",
		ProviderKey:     "openai",
		Model:           "safe-model",
		Direction:       detection.DirectionOutput,
		Text:            "model output",
		Execution:       routing.ExecutionAsyncShadow,
		FallbackAction:  routing.ActionAllow,
		VerdictProvider: provider,
	})

	if result.Verdict.SuggestedAction != "" {
		t.Fatalf("inline result verdict = %+v, want no verdict for async shadow", result.Verdict)
	}
	if result.Decision.Action != policy.ActionAllow {
		t.Fatalf("inline decision = %q, want snapshot allow", result.Decision.Action)
	}

	select {
	case shadow := <-shadowResults:
		if shadow.Verdict.SuggestedAction != verdict.ActionBlock {
			t.Fatalf("shadow verdict action = %q, want block", shadow.Verdict.SuggestedAction)
		}
		if shadow.Direction != detection.DirectionOutput || shadow.Text != "model output" {
			t.Fatalf("shadow identity = %+v, want output text", shadow)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async shadow evaluation")
	}
}

func TestTrafficEvaluationUseCaseFallsBackWhenVerdictFails(t *testing.T) {
	useCase := TrafficEvaluationUseCase{}
	provider := verdict.ProviderFunc(func(context.Context, verdict.Request) (verdict.Result, error) {
		return verdict.Result{}, errors.New("verdict unavailable")
	})

	result := useCase.Evaluate(context.Background(), TrafficEvaluationInput{
		Direction:       detection.DirectionInput,
		Execution:       routing.ExecutionInline,
		FallbackAction:  routing.ActionBlock,
		VerdictProvider: provider,
	})

	if result.Decision.Action != policy.ActionBlock {
		t.Fatalf("decision action = %q, want fallback block", result.Decision.Action)
	}
	if !result.Decision.ModelSignal.FallbackUsed {
		t.Fatal("fallback used = false, want true")
	}
}

func TestTrafficEvaluationUseCaseDefaultsFallbackToBlockOnVerdictError(t *testing.T) {
	useCase := TrafficEvaluationUseCase{}
	provider := verdict.ProviderFunc(func(context.Context, verdict.Request) (verdict.Result, error) {
		return verdict.Result{}, errors.New("verdict unavailable")
	})

	result := useCase.Evaluate(context.Background(), TrafficEvaluationInput{
		Direction:       detection.DirectionOutput,
		Execution:       routing.ExecutionInline,
		VerdictProvider: provider,
	})

	if result.Decision.Action != policy.ActionBlock {
		t.Fatalf("decision action = %q, want default fallback block", result.Decision.Action)
	}
	if result.Decision.ModelSignal.FallbackAction != policy.ActionBlock {
		t.Fatalf("fallback action = %q, want block", result.Decision.ModelSignal.FallbackAction)
	}
}

func TestTrafficEvaluationUseCaseSkipsMissingVerdictProvider(t *testing.T) {
	useCase := TrafficEvaluationUseCase{}

	result := useCase.Evaluate(context.Background(), TrafficEvaluationInput{
		Direction:      detection.DirectionInput,
		Execution:      routing.ExecutionInline,
		FallbackAction: routing.ActionBlock,
	})

	if result.Verdict.SuggestedAction != "" {
		t.Fatalf("verdict = %+v, want zero verdict without provider", result.Verdict)
	}
	if result.Decision.Action != policy.ActionAllow {
		t.Fatalf("decision action = %q, want allow without provider signal", result.Decision.Action)
	}
}

func TestTrafficEvaluationUseCaseEvaluateVerdictUsesTimeout(t *testing.T) {
	useCase := TrafficEvaluationUseCase{VerdictTimeout: time.Nanosecond}
	provider := verdict.ProviderFunc(func(ctx context.Context, _ verdict.Request) (verdict.Result, error) {
		<-ctx.Done()
		return verdict.Result{}, ctx.Err()
	})

	_, err := useCase.evaluateVerdict(context.Background(), TrafficEvaluationInput{
		VerdictProvider: provider,
	})

	if err == nil {
		t.Fatal("evaluateVerdict() error = nil, want timeout error")
	}
}

func TestTrafficEvaluationUseCaseEvaluatePolicyOnlySkipsVerdict(t *testing.T) {
	useCase := TrafficEvaluationUseCase{
		Snapshot: fakePolicySnapshot{decision: policy.Decision{Action: policy.ActionAllow}},
	}

	result := useCase.EvaluatePolicyOnly(TrafficEvaluationInput{
		RouteKey:    "chat",
		ProviderKey: "openai",
		Model:       "safe-model",
		Direction:   detection.DirectionOutput,
		Text:        "model output",
		VerdictProvider: verdict.ProviderFunc(func(context.Context, verdict.Request) (verdict.Result, error) {
			t.Fatal("verdict provider should not be called")
			return verdict.Result{}, nil
		}),
	})

	if result.Direction != detection.DirectionOutput || result.Text != "model output" {
		t.Fatalf("result identity = %+v, want output model text", result)
	}
	if result.Decision.Action != policy.ActionAllow {
		t.Fatalf("decision action = %q, want allow", result.Decision.Action)
	}
	if result.Verdict.SuggestedAction != "" {
		t.Fatalf("verdict = %+v, want zero verdict", result.Verdict)
	}
}

type shadowObserverFunc func(context.Context, TrafficEvaluationResult)

func (f shadowObserverFunc) ObserveShadowEvaluation(ctx context.Context, result TrafficEvaluationResult) {
	f(ctx, result)
}
