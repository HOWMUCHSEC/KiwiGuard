package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClickHouseRetentionMaintainerAppliesShortestClickHousePolicy(t *testing.T) {
	executor := &recordingClickHouseExecutor{}
	maintainer := newClickHouseRetentionMaintainer(executor)
	cfg := RuntimeConfig{
		Sinks: []SinkConfig{{
			Key:  "events",
			Kind: "clickhouse",
		}},
		Retention: []RetentionPolicyConfig{
			{Key: "events-30d", SinkKey: "events", EventType: "*", RetentionDays: 30},
			{Key: "events-7d", SinkKey: "events", EventType: "*", RetentionDays: 7},
		},
	}

	if err := maintainer.ApplyRetentionPolicies(context.Background(), cfg); err != nil {
		t.Fatalf("ApplyRetentionPolicies() error = %v", err)
	}
	if executor.query == "" {
		t.Fatal("retention query was not executed")
	}
	if !strings.Contains(executor.query, "INTERVAL 7 DAY") {
		t.Fatalf("retention query = %q, want shortest retention interval", executor.query)
	}
}

func TestClickHouseRetentionMaintainerRejectsUnsupportedSinkKind(t *testing.T) {
	maintainer := newClickHouseRetentionMaintainer(&recordingClickHouseExecutor{})
	cfg := RuntimeConfig{
		Sinks: []SinkConfig{{
			Key:  "webhook-events",
			Kind: "webhook",
		}},
		Retention: []RetentionPolicyConfig{{
			Key:           "webhook-30d",
			SinkKey:       "webhook-events",
			EventType:     "*",
			RetentionDays: 30,
		}},
	}

	err := maintainer.ApplyRetentionPolicies(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "unsupported retention sink") {
		t.Fatalf("ApplyRetentionPolicies() error = %v, want unsupported sink error", err)
	}
}

func TestClickHouseRetentionMaintainerRequiresExecutor(t *testing.T) {
	maintainer := newClickHouseRetentionMaintainer(nil)

	err := maintainer.ApplyRetentionPolicies(context.Background(), RuntimeConfig{})
	if err == nil || !strings.Contains(err.Error(), "executor is required") {
		t.Fatalf("ApplyRetentionPolicies() error = %v, want executor error", err)
	}
}

func TestClickHouseRetentionMaintainerReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	maintainer := newClickHouseRetentionMaintainer(&recordingClickHouseExecutor{})

	err := maintainer.ApplyRetentionPolicies(ctx, RuntimeConfig{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyRetentionPolicies() error = %v, want context canceled", err)
	}
}

func TestClickHouseRetentionMaintainerSkipsWhenNoClickHousePolicyApplies(t *testing.T) {
	executor := &recordingClickHouseExecutor{}
	maintainer := newClickHouseRetentionMaintainer(executor)
	cfg := RuntimeConfig{
		Sinks: []SinkConfig{{
			Key:      "disabled-events",
			Kind:     "webhook",
			Disabled: true,
		}},
		Retention: []RetentionPolicyConfig{{
			Key:           "global-30d",
			EventType:     "*",
			RetentionDays: 30,
		}},
	}

	if err := maintainer.ApplyRetentionPolicies(context.Background(), cfg); err != nil {
		t.Fatalf("ApplyRetentionPolicies() error = %v", err)
	}
	if executor.query != "" {
		t.Fatalf("retention query = %q, want no query", executor.query)
	}
}

func TestShortestClickHouseRetentionDaysRejectsInvalidPolicies(t *testing.T) {
	tests := []struct {
		name string
		cfg  RuntimeConfig
		want string
	}{
		{
			name: "invalid retention days",
			cfg: RuntimeConfig{Retention: []RetentionPolicyConfig{{
				Key:           "broken",
				RetentionDays: 0,
			}}},
			want: "invalid retention days",
		},
		{
			name: "unknown sink",
			cfg: RuntimeConfig{
				Sinks: []SinkConfig{{Key: "events", Kind: "clickhouse"}},
				Retention: []RetentionPolicyConfig{{
					Key:           "missing-sink",
					SinkKey:       "missing",
					RetentionDays: 30,
				}},
			},
			want: "references unknown sink",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := shortestClickHouseRetentionDays(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("shortestClickHouseRetentionDays() error = %v, want %q", err, tt.want)
			}
		})
	}
}

type recordingClickHouseExecutor struct {
	query string
	err   error
}

func (e *recordingClickHouseExecutor) Exec(ctx context.Context, query string, args ...any) error {
	e.query = query
	return e.err
}

func TestClickHouseRetentionMaintainerWrapsExecutorError(t *testing.T) {
	wantErr := errors.New("alter failed")
	maintainer := newClickHouseRetentionMaintainer(&recordingClickHouseExecutor{err: wantErr})

	err := maintainer.ApplyRetentionPolicies(context.Background(), RuntimeConfig{
		Sinks:     []SinkConfig{{Key: "events", Kind: "clickhouse"}},
		Retention: []RetentionPolicyConfig{{Key: "events-30d", SinkKey: "events", RetentionDays: 30}},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApplyRetentionPolicies() error = %v, want %v", err, wantErr)
	}
}
