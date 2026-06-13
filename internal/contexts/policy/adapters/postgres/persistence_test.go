package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLoadBundlesHydratesPolicyGraph(t *testing.T) {
	raw := json.RawMessage(`{"kind":"email","categories":["pii.email"]}`)
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"bundle-id", "pii", "user", "2026.06", "PII", "block", true, json.RawMessage(`{}`)}}),
			newRows([][]any{{"bundle-id", "detector-id", "email", "builtin", "[a-z]+", raw, true}}),
			newRows([][]any{{"bundle-id", "rule-id", "email-rule", "detect email", "high", "block", true, 10, json.RawMessage(`{}`)}}),
			newRows([][]any{{"rule-id", "email"}}),
			newRows([][]any{{"rule-id", "route-id", "provider-id", "gpt-4.1", "request"}}),
		},
	}

	bundles, err := LoadBundles(context.Background(), q, "revision-id")
	if err != nil {
		t.Fatalf("LoadBundles() error = %v", err)
	}

	want := []Bundle{{
		ID:            "bundle-id",
		Key:           "pii",
		Source:        "user",
		Version:       "2026.06",
		Description:   "PII",
		DefaultAction: "block",
		Enabled:       true,
		Metadata:      json.RawMessage(`{}`),
		Detectors: []Detector{{
			ID:         "detector-id",
			Key:        "email",
			Kind:       "email",
			Pattern:    "[a-z]+",
			Categories: []string{"pii.email"},
			Config:     raw,
			Enabled:    true,
		}},
		Rules: []Rule{{
			ID:           "rule-id",
			Key:          "email-rule",
			Description:  "detect email",
			Severity:     "high",
			Action:       "block",
			Enabled:      true,
			Priority:     10,
			Config:       json.RawMessage(`{}`),
			DetectorKeys: []string{"email"},
			Scopes: []RuleScope{{
				RouteID:    "route-id",
				ProviderID: "provider-id",
				Model:      "gpt-4.1",
				Direction:  "input",
			}},
		}},
	}}
	if !reflect.DeepEqual(bundles, want) {
		t.Fatalf("LoadBundles() = %#v, want %#v", bundles, want)
	}
}

func TestLoadBundlesByKeysSkipsEmptyKeys(t *testing.T) {
	bundles, err := LoadBundlesByKeys(context.Background(), &recordingQueryer{}, "revision-id", nil)
	if err != nil {
		t.Fatalf("LoadBundlesByKeys(empty) error = %v", err)
	}
	if len(bundles) != 0 {
		t.Fatalf("LoadBundlesByKeys(empty) length = %d, want 0", len(bundles))
	}
}

func TestLoadBundlesByKeysHydratesMatchingBundles(t *testing.T) {
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"bundle-id", "safe", "system", "2026.06", "", "allow", true, json.RawMessage(`{"owner":"kiwi"}`)}}),
			newRows(nil),
			newRows(nil),
		},
	}

	bundles, err := LoadBundlesByKeys(context.Background(), q, "revision-id", []string{"safe"})
	if err != nil {
		t.Fatalf("LoadBundlesByKeys() error = %v", err)
	}

	if len(bundles) != 1 {
		t.Fatalf("LoadBundlesByKeys() length = %d, want 1", len(bundles))
	}
	if bundles[0].Key != "safe" || bundles[0].Source != "system" || string(bundles[0].Metadata) != `{"owner":"kiwi"}` {
		t.Fatalf("LoadBundlesByKeys() bundle = %#v", bundles[0])
	}
}

func TestLoadBundlesPropagatesRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		q       *recordingQueryer
		wantErr string
	}{
		{
			name:    "bundle query",
			q:       &recordingQueryer{queryErr: errors.New("query failed")},
			wantErr: "load policy bundles: query failed",
		},
		{
			name: "bundle scan",
			q: &recordingQueryer{queryRows: []pgx.Rows{
				newRows([][]any{{"bundle-id"}}),
			}},
			wantErr: "scan policy bundle:",
		},
		{
			name: "bundle iterate",
			q: &recordingQueryer{queryRows: []pgx.Rows{
				rowsWithErr(errors.New("cursor failed")),
			}},
			wantErr: "iterate policy bundles: cursor failed",
		},
		{
			name: "detector query",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows([][]any{{"bundle-id", "pii", "user", "2026.06", "", "allow", true, json.RawMessage(`{}`)}}),
				},
				queryErrs: []error{nil, errors.New("detector query failed")},
			},
			wantErr: "load policy detectors: detector query failed",
		},
		{
			name: "rule detector query",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows([][]any{{"bundle-id", "pii", "user", "2026.06", "", "allow", true, json.RawMessage(`{}`)}}),
					newRows(nil),
					newRows([][]any{{"bundle-id", "rule-id", "rule", "", "low", "allow", true, 1, json.RawMessage(`{}`)}}),
				},
				queryErrs: []error{nil, nil, nil, errors.New("rule detector query failed")},
			},
			wantErr: "load policy rule detectors: rule detector query failed",
		},
		{
			name: "scope query",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows([][]any{{"bundle-id", "pii", "user", "2026.06", "", "allow", true, json.RawMessage(`{}`)}}),
					newRows(nil),
					newRows([][]any{{"bundle-id", "rule-id", "rule", "", "low", "allow", true, 1, json.RawMessage(`{}`)}}),
					newRows(nil),
				},
				queryErrs: []error{nil, nil, nil, nil, errors.New("scope query failed")},
			},
			wantErr: "load policy rule scopes: scope query failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadBundles(context.Background(), tt.q, "revision-id")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("LoadBundles() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateDraftBundleActivationWritesRequestedKeys(t *testing.T) {
	q := &recordingQueryer{}

	if err := UpdateDraftBundleActivation(context.Background(), q, "revision-id", []string{"pii", "secrets"}); err != nil {
		t.Fatalf("UpdateDraftBundleActivation() error = %v", err)
	}

	if len(q.execArgs) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(q.execArgs))
	}
	assertArg(t, q.execArgs[0], 0, "revision-id")
	keys, ok := q.execArgs[0][1].([]string)
	if !ok || !reflect.DeepEqual(keys, []string{"pii", "secrets"}) {
		t.Fatalf("activation keys = %#v, want pii and secrets", q.execArgs[0][1])
	}
}

func TestUpdateDraftBundleActivationPropagatesExecError(t *testing.T) {
	q := &recordingQueryer{execErr: errors.New("activation failed")}

	err := UpdateDraftBundleActivation(context.Background(), q, "revision-id", []string{"pii"})
	if err == nil || !strings.Contains(err.Error(), "update policy bundle activation state: activation failed") {
		t.Fatalf("UpdateDraftBundleActivation() error = %v, want activation failure", err)
	}
}

func TestUpsertBundleWritesPolicyGraph(t *testing.T) {
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithValues("bundle-id"),
			rowWithValues("detector-id"),
			rowWithValues("rule-id"),
		},
	}

	err := UpsertBundle(context.Background(), q, "revision-id", Bundle{
		Key:     "pii",
		Version: "2026.06",
		Enabled: true,
		Detectors: []Detector{{
			Key:        "email",
			Kind:       "email",
			Categories: []string{"pii.email"},
			Enabled:    true,
		}},
		Rules: []Rule{{
			Key:          "email-rule",
			Severity:     "high",
			Action:       "block",
			Enabled:      true,
			Priority:     10,
			DetectorKeys: []string{"email"},
			Scopes: []RuleScope{{
				Model:     "gpt-4.1",
				Direction: "output",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("UpsertBundle() error = %v", err)
	}

	if len(q.queryRowArgs) != 3 {
		t.Fatalf("QueryRow calls = %d, want 3", len(q.queryRowArgs))
	}
	assertArg(t, q.queryRowArgs[0], 0, "revision-id")
	assertArg(t, q.queryRowArgs[0], 2, "user")
	assertArg(t, q.queryRowArgs[0], 5, "allow")
	assertArg(t, q.queryRowArgs[1], 2, "builtin")
	if len(q.execArgs) != 4 {
		t.Fatalf("Exec calls = %d, want deletes plus rule detector and scope", len(q.execArgs))
	}
	assertArg(t, q.execArgs[3], 4, "response")
}

func TestUpsertBundlePreservesExplicitValuesAndNormalizesScopes(t *testing.T) {
	customDetectorConfig := json.RawMessage(`{"custom":true}`)
	customRuleConfig := json.RawMessage(`{"threshold":2}`)
	metadata := json.RawMessage(`{"labels":["managed"]}`)
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithValues("bundle-id"),
			rowWithValues("detector-id"),
			rowWithValues("rule-id"),
		},
	}

	err := UpsertBundle(context.Background(), q, "revision-id", Bundle{
		Key:           "secrets",
		Source:        "system",
		DefaultAction: "block",
		Metadata:      metadata,
		Detectors: []Detector{{
			Key:     "regex",
			Kind:    "regex",
			Pattern: "token-[0-9]+",
			Config:  customDetectorConfig,
			Enabled: true,
		}},
		Rules: []Rule{{
			Key:          "regex-rule",
			Config:       customRuleConfig,
			DetectorKeys: []string{"regex"},
			Scopes: []RuleScope{{
				RouteID:    "route-id",
				ProviderID: "provider-id",
				Direction:  "both",
			}, {
				Direction: "nonsense",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("UpsertBundle() error = %v", err)
	}

	assertArg(t, q.queryRowArgs[0], 2, "system")
	assertArg(t, q.queryRowArgs[0], 5, "block")
	assertRawMessageArg(t, q.queryRowArgs[0], 7, metadata)
	assertArg(t, q.queryRowArgs[1], 2, "regex")
	assertRawMessageArg(t, q.queryRowArgs[1], 4, customDetectorConfig)
	assertRawMessageArg(t, q.queryRowArgs[2], 7, customRuleConfig)
	if len(q.execArgs) != 5 {
		t.Fatalf("Exec calls = %d, want deletes plus rule detector and two scopes", len(q.execArgs))
	}
	assertArg(t, q.execArgs[3], 4, "both")
	assertArg(t, q.execArgs[4], 4, "both")
}

func TestUpsertBundlePropagatesRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		q       *recordingQueryer
		bundle  Bundle
		wantErr string
	}{
		{
			name: "bundle upsert",
			q: &recordingQueryer{rowsForQueryRow: []pgx.Row{
				rowWithErr(errors.New("bundle insert failed")),
			}},
			bundle:  Bundle{Key: "pii"},
			wantErr: "upsert policy bundle: bundle insert failed",
		},
		{
			name: "detector delete",
			q: &recordingQueryer{
				rowsForQueryRow: []pgx.Row{rowWithValues("bundle-id")},
				execErrs:        []error{errors.New("delete detectors failed")},
			},
			bundle:  Bundle{Key: "pii"},
			wantErr: "replace policy detectors: delete detectors failed",
		},
		{
			name: "rule delete",
			q: &recordingQueryer{
				rowsForQueryRow: []pgx.Row{rowWithValues("bundle-id")},
				execErrs:        []error{nil, errors.New("delete rules failed")},
			},
			bundle:  Bundle{Key: "pii"},
			wantErr: "replace policy rules: delete rules failed",
		},
		{
			name: "detector insert",
			q: &recordingQueryer{rowsForQueryRow: []pgx.Row{
				rowWithValues("bundle-id"),
				rowWithErr(errors.New("detector insert failed")),
			}},
			bundle: Bundle{Key: "pii", Detectors: []Detector{{
				Key: "email",
			}}},
			wantErr: "insert policy detector: detector insert failed",
		},
		{
			name: "rule insert",
			q: &recordingQueryer{rowsForQueryRow: []pgx.Row{
				rowWithValues("bundle-id"),
				rowWithErr(errors.New("rule insert failed")),
			}},
			bundle: Bundle{Key: "pii", Rules: []Rule{{
				Key: "rule",
			}}},
			wantErr: "insert policy rule: rule insert failed",
		},
		{
			name: "rule detector insert",
			q: &recordingQueryer{
				rowsForQueryRow: []pgx.Row{
					rowWithValues("bundle-id"),
					rowWithValues("detector-id"),
					rowWithValues("rule-id"),
				},
				execErrs: []error{nil, nil, errors.New("join insert failed")},
			},
			bundle: Bundle{Key: "pii", Detectors: []Detector{{
				Key: "email",
			}}, Rules: []Rule{{
				Key:          "rule",
				DetectorKeys: []string{"email"},
			}}},
			wantErr: "insert policy rule detector: join insert failed",
		},
		{
			name: "scope insert",
			q: &recordingQueryer{
				rowsForQueryRow: []pgx.Row{
					rowWithValues("bundle-id"),
					rowWithValues("rule-id"),
				},
				execErrs: []error{nil, nil, errors.New("scope insert failed")},
			},
			bundle: Bundle{Key: "pii", Rules: []Rule{{
				Key:    "rule",
				Scopes: []RuleScope{{Direction: "input"}},
			}}},
			wantErr: "insert policy rule scope: scope insert failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpsertBundle(context.Background(), tt.q, "revision-id", tt.bundle)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("UpsertBundle() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestUpsertBundleRejectsUnknownRuleDetector(t *testing.T) {
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithValues("bundle-id"),
			rowWithValues("rule-id"),
		},
	}

	err := UpsertBundle(context.Background(), q, "revision-id", Bundle{
		Key: "pii",
		Rules: []Rule{{
			Key:          "rule",
			DetectorKeys: []string{"missing"},
		}},
	})
	if err == nil {
		t.Fatal("UpsertBundle() error = nil, want missing detector error")
	}
}

func TestCloneGraphAndBindingsClonePolicyRecords(t *testing.T) {
	raw := json.RawMessage(`{}`)
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"old-bundle-id", "pii", "user", "2026.06", "PII", "block", true, raw}}),
			newRows([][]any{{"old-detector-id", "old-bundle-id", "email", "regex", "[a-z]+", raw, true}}),
			newRows([][]any{{"old-rule-id", "old-bundle-id", "email-rule", "detect email", "high", "block", true, 10, raw}}),
			newRows([][]any{{"old-rule-id", "old-detector-id", "all"}}),
			newRows([][]any{{"old-rule-id", "old-route-id", "old-provider-id", "gpt-4.1", "request"}}),
			newRows([][]any{{"old-route-id", "old-bundle-id", true, 10}}),
		},
		rowsForQueryRow: []pgx.Row{
			rowWithValues("new-bundle-id"),
			rowWithValues("new-detector-id"),
			rowWithValues("new-rule-id"),
		},
	}

	bundleIDs, detectorIDs, ruleIDs, err := CloneGraph(context.Background(), q, "source-revision-id", "draft-revision-id")
	if err != nil {
		t.Fatalf("CloneGraph() error = %v", err)
	}
	if err := CloneBindings(context.Background(), q, "source-revision-id", ruleIDs, detectorIDs, map[string]string{"old-route-id": "new-route-id"}, map[string]string{"old-provider-id": "new-provider-id"}, bundleIDs); err != nil {
		t.Fatalf("CloneBindings() error = %v", err)
	}

	if bundleIDs["old-bundle-id"] != "new-bundle-id" {
		t.Fatalf("bundle ID map = %#v", bundleIDs)
	}
	if detectorIDs["old-detector-id"] != "new-detector-id" {
		t.Fatalf("detector ID map = %#v", detectorIDs)
	}
	if ruleIDs["old-rule-id"] != "new-rule-id" {
		t.Fatalf("rule ID map = %#v", ruleIDs)
	}
	if len(q.execArgs) != 3 {
		t.Fatalf("Exec calls = %d, want policy rule detector, scope, route binding", len(q.execArgs))
	}
	assertArg(t, q.execArgs[0], 0, "new-rule-id")
	assertArg(t, q.execArgs[0], 1, "new-detector-id")
	assertArg(t, q.execArgs[1], 1, "new-route-id")
	assertArg(t, q.execArgs[1], 2, "new-provider-id")
	assertArg(t, q.execArgs[2], 0, "new-route-id")
	assertArg(t, q.execArgs[2], 1, "new-bundle-id")
}

func TestCloneGraphPropagatesRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		q       *recordingQueryer
		wantErr string
	}{
		{
			name:    "bundle query",
			q:       &recordingQueryer{queryErr: errors.New("bundle query failed")},
			wantErr: "load policy bundles for draft clone: bundle query failed",
		},
		{
			name: "bundle scan",
			q: &recordingQueryer{queryRows: []pgx.Rows{
				newRows([][]any{{"old-bundle-id"}}),
			}},
			wantErr: "scan policy bundle for draft clone:",
		},
		{
			name: "bundle insert",
			q: &recordingQueryer{
				queryRows:       []pgx.Rows{newRows([][]any{{"old-bundle-id", "pii", "user", "", "", "allow", true, json.RawMessage(`{}`)}})},
				rowsForQueryRow: []pgx.Row{rowWithErr(errors.New("clone bundle failed"))},
			},
			wantErr: "clone policy bundle pii: clone bundle failed",
		},
		{
			name: "detector missing bundle remap",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows([][]any{{"old-detector-id", "missing-bundle-id", "email", "builtin", "", json.RawMessage(`{}`), true}}),
				},
			},
			wantErr: "missing cloned policy detector bundle id for missing-bundle-id",
		},
		{
			name: "rule missing bundle remap",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows(nil),
					newRows([][]any{{"old-rule-id", "missing-bundle-id", "rule", "", "low", "allow", true, 1, json.RawMessage(`{}`)}}),
				},
			},
			wantErr: "missing cloned policy rule bundle id for missing-bundle-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := CloneGraph(context.Background(), tt.q, "source-revision-id", "draft-revision-id")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CloneGraph() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCloneBindingsPropagatesRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		q       *recordingQueryer
		rules   map[string]string
		dets    map[string]string
		routes  map[string]string
		provs   map[string]string
		bundles map[string]string
		wantErr string
	}{
		{
			name:    "rule detector query",
			q:       &recordingQueryer{queryErr: errors.New("rule detector query failed")},
			wantErr: "load policy rule detectors for draft clone: rule detector query failed",
		},
		{
			name: "rule detector missing rule remap",
			q: &recordingQueryer{queryRows: []pgx.Rows{
				newRows([][]any{{"old-rule-id", "old-detector-id", "all"}}),
			}},
			dets:    map[string]string{"old-detector-id": "new-detector-id"},
			wantErr: "missing cloned policy rule detector rule id for old-rule-id",
		},
		{
			name: "rule detector missing detector remap",
			q: &recordingQueryer{queryRows: []pgx.Rows{
				newRows([][]any{{"old-rule-id", "old-detector-id", "all"}}),
			}},
			rules:   map[string]string{"old-rule-id": "new-rule-id"},
			wantErr: "missing cloned policy rule detector detector id for old-detector-id",
		},
		{
			name: "scope optional route missing remap",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows([][]any{{"old-rule-id", "old-route-id", "", "gpt-4.1", "request"}}),
				},
			},
			rules:   map[string]string{"old-rule-id": "new-rule-id"},
			wantErr: "missing cloned policy rule scope route id for old-route-id",
		},
		{
			name: "scope optional provider missing remap",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows([][]any{{"old-rule-id", "", "old-provider-id", "gpt-4.1", "request"}}),
				},
			},
			rules:   map[string]string{"old-rule-id": "new-rule-id"},
			wantErr: "missing cloned policy rule scope provider id for old-provider-id",
		},
		{
			name: "route binding missing bundle remap",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows(nil),
					newRows([][]any{{"old-route-id", "old-bundle-id", true, 1}}),
				},
			},
			routes:  map[string]string{"old-route-id": "new-route-id"},
			wantErr: "missing cloned route policy binding bundle id for old-bundle-id",
		},
		{
			name: "route binding insert error",
			q: &recordingQueryer{
				queryRows: []pgx.Rows{
					newRows(nil),
					newRows(nil),
					newRows([][]any{{"old-route-id", "old-bundle-id", true, 1}}),
				},
				execErr: errors.New("route binding insert failed"),
			},
			routes:  map[string]string{"old-route-id": "new-route-id"},
			bundles: map[string]string{"old-bundle-id": "new-bundle-id"},
			wantErr: "clone route policy binding: route binding insert failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneBindings(context.Background(), tt.q, "source-revision-id", tt.rules, tt.dets, tt.routes, tt.provs, tt.bundles)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CloneBindings() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyConversionHelpers(t *testing.T) {
	config := detectorConfig("email", []string{"pii.email"})
	if got := detectorStorageKind("email"); got != "builtin" {
		t.Fatalf("detectorStorageKind(email) = %q, want builtin", got)
	}
	if got := detectorPolicyKind("builtin", config); got != "email" {
		t.Fatalf("detectorPolicyKind() = %q, want email", got)
	}
	if got := detectorCategories(config); len(got) != 1 || got[0] != "pii.email" {
		t.Fatalf("detectorCategories() = %#v, want pii.email", got)
	}
	if got := storageDirection("input"); got != "request" {
		t.Fatalf("storageDirection(input) = %q, want request", got)
	}
	if got := policyDirection("response"); got != "output" {
		t.Fatalf("policyDirection(response) = %q, want output", got)
	}
}

func TestPolicyConversionFallbacks(t *testing.T) {
	if got := detectorStorageKind("custom"); got != "custom" {
		t.Fatalf("detectorStorageKind(custom) = %q, want custom", got)
	}
	if got := detectorPolicyKind("builtin", json.RawMessage(`{`)); got != "builtin" {
		t.Fatalf("detectorPolicyKind(invalid builtin config) = %q, want builtin", got)
	}
	if got := detectorPolicyKind("regex", json.RawMessage(`{"kind":"email"}`)); got != "regex" {
		t.Fatalf("detectorPolicyKind(regex) = %q, want regex", got)
	}
	if got := detectorCategories(json.RawMessage(`{`)); got != nil {
		t.Fatalf("detectorCategories(invalid) = %#v, want nil", got)
	}
	if got := storageDirection("request"); got != "request" {
		t.Fatalf("storageDirection(request) = %q, want request", got)
	}
	if got := storageDirection("weird"); got != "both" {
		t.Fatalf("storageDirection(weird) = %q, want both", got)
	}
	if got := policyDirection("both"); got != "" {
		t.Fatalf("policyDirection(both) = %q, want empty", got)
	}
	if got := defaultBundleSource("system"); got != "system" {
		t.Fatalf("defaultBundleSource(system) = %q, want system", got)
	}
	if got := defaultBundleAction("block"); got != "block" {
		t.Fatalf("defaultBundleAction(block) = %q, want block", got)
	}
	raw := json.RawMessage(`{"ok":true}`)
	if got := defaultJSONObject(raw); !reflect.DeepEqual(got, raw) {
		t.Fatalf("defaultJSONObject(raw) = %s, want %s", got, raw)
	}
	if got := defaultJSONObject(nil); string(got) != `{}` {
		t.Fatalf("defaultJSONObject(nil) = %s, want {}", got)
	}
	if got, err := remapOptionalID("", nil, "thing"); err != nil || got != "" {
		t.Fatalf("remapOptionalID(empty) = %q, %v; want empty nil", got, err)
	}
}

func assertArg[T comparable](t *testing.T, args []any, index int, want T) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("args length = %d, need index %d", len(args), index)
	}
	got, ok := args[index].(T)
	if !ok {
		t.Fatalf("arg[%d] type = %T, want %T", index, args[index], want)
	}
	if got != want {
		t.Fatalf("arg[%d] = %v, want %v", index, got, want)
	}
}

func assertRawMessageArg(t *testing.T, args []any, index int, want json.RawMessage) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("args length = %d, need index %d", len(args), index)
	}
	got, ok := args[index].(json.RawMessage)
	if !ok {
		t.Fatalf("arg[%d] type = %T, want json.RawMessage", index, args[index])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arg[%d] = %s, want %s", index, got, want)
	}
}

type recordingQueryer struct {
	queryRows       []pgx.Rows
	queryErr        error
	queryErrs       []error
	rowsForQueryRow []pgx.Row
	queryRowArgs    [][]any
	execArgs        [][]any
	execErr         error
	execErrs        []error
}

func (q *recordingQueryer) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	if len(q.queryErrs) > 0 {
		err := q.queryErrs[0]
		q.queryErrs = q.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	if len(q.queryRows) == 0 {
		panic("unexpected Query")
	}
	rows := q.queryRows[0]
	q.queryRows = q.queryRows[1:]
	return rows, nil
}

func (q *recordingQueryer) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	q.queryRowArgs = append(q.queryRowArgs, args)
	if len(q.rowsForQueryRow) == 0 {
		panic("unexpected QueryRow")
	}
	row := q.rowsForQueryRow[0]
	q.rowsForQueryRow = q.rowsForQueryRow[1:]
	return row
}

func (q *recordingQueryer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	q.execArgs = append(q.execArgs, args)
	if len(q.execErrs) > 0 {
		err := q.execErrs[0]
		q.execErrs = q.execErrs[1:]
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	if q.execErr != nil {
		return pgconn.CommandTag{}, q.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
}

func newRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func rowsWithErr(err error) *fakeRows {
	return &fakeRows{index: -1, err: err}
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT 1")
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	return assignValues(dest, r.values[r.index])
}

func (r *fakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.values) {
		return nil, errors.New("values without current row")
	}
	return r.values[r.index], nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

type fakeRow struct {
	values []any
	err    error
}

func rowWithValues(values ...any) fakeRow {
	return fakeRow{values: values}
}

func rowWithErr(err error) fakeRow {
	return fakeRow{err: err}
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignValues(dest, r.values)
}

func assignValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan destination count does not match values")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("scan destination must be a non-nil pointer")
		}
		value := reflect.ValueOf(values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("scan value type is not assignable")
		}
		target.Elem().Set(value)
	}
	return nil
}
