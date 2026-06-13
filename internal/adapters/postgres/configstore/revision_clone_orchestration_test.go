package configstore

import (
	"context"
	"strings"
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	"github.com/jackc/pgx/v5"
)

func TestEnsureDraftRevisionClonesActiveRevisionGraph(t *testing.T) {
	ctx := context.Background()
	tx := &fakeConfigTx{
		rowResults: []fakeConfigRow{
			rowWithConfigErr(pgx.ErrNoRows),
			rowWithConfigValues("active-1"),
			rowWithConfigValues("draft-1"),
			rowWithConfigValues("provider-new"),
			rowWithConfigValues("mapping-new"),
			rowWithConfigValues("route-new"),
			rowWithConfigValues("verdict-new"),
			rowWithConfigValues("bundle-new"),
			rowWithConfigValues("detector-new"),
			rowWithConfigValues("rule-new"),
			rowWithConfigValues("sink-new"),
		},
		rows: []*fakeConfigRows{
			newConfigRows([][]any{{
				"provider-old", "openai", "http://upstream.test", "secret/openai", 30000,
				"openai_compatible", []byte(`{"x-test":"true"}`), []byte(`{}`), []byte(`{}`), []byte(`{"chat":true}`),
			}}),
			newConfigRows([][]any{{
				"mapping-old", "default", "gpt-test", "provider-old", "gpt-4o-mini", []byte(`{"temperature":0}`),
			}}),
			newConfigRows([][]any{{
				"route-old", "chat", "/v1/chat/completions", "openai", "gpt-4o-mini", "inline",
				"allow", true, 10, "POST", "/v1/chat/completions", "mapping-old",
			}}),
			newConfigRows([][]any{{
				"route-old", 120, 60, 8, int64(1048576), true,
			}}),
			newConfigRows([][]any{{
				"client-1", "route-old", 40, 60, 3, int64(262144), true,
			}}),
			newConfigRows([][]any{{
				"verdict-old", "sec-model", "http", "http://verdict.test/evaluate", 5000, "secret/verdict",
				[]byte(`{}`), "kg-sec", []byte(`{}`), []byte(`{}`), 16, true,
			}}),
			newConfigRows([][]any{{
				"bundle-old", "pii", "user", "2026.05", "PII rules", "block", true, []byte(`{"owner":"security"}`),
			}}),
			newConfigRows([][]any{{
				"detector-old", "bundle-old", "email", "regex", "[a-z]+@[a-z]+\\.com", []byte(`{"categories":["pii.email"]}`), true,
			}}),
			newConfigRows([][]any{{
				"rule-old", "bundle-old", "block-email", "block email", "high", "block", true, 10, []byte(`{}`),
			}}),
			newConfigRows([][]any{{
				"rule-old", "detector-old", "any",
			}}),
			newConfigRows([][]any{{
				"rule-old", "route-old", "provider-old", "gpt-test", "request",
			}}),
			newConfigRows([][]any{{
				"route-old", "bundle-old", true, 10,
			}}),
			newConfigRows([][]any{{
				"route-old", "verdict-old", true, "inline", 10,
			}}),
			newConfigRows([][]any{{
				"sink-old", "events", "clickhouse", true, []byte(`{"database":"kiwiguard"}`),
			}}),
			newConfigRows([][]any{{
				"events-30d", "sink-old", "*", 30,
			}}),
			newConfigRows([][]any{{
				"redacted-sample", "route-old", "both", true, 0.25, "redacted",
			}}),
		},
	}

	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}
	var id string
	err := repo.WithDraftRevision(ctx, "clone active revision", func(_ context.Context, _ revisionstore.Queryer, revisionID string) error {
		id = revisionID
		return nil
	})
	if err != nil {
		t.Fatalf("WithDraftRevision() error = %v", err)
	}
	if id != "draft-1" {
		t.Fatalf("WithDraftRevision() revision = %q, want draft-1", id)
	}
	if len(tx.rows) != 0 || len(tx.rowResults) != 0 {
		t.Fatalf("clone left unread fake data: rows=%d rowResults=%d", len(tx.rows), len(tx.rowResults))
	}

	for _, want := range []string{
		"insert into providers",
		"insert into model_mappings",
		"insert into routes",
		"insert into verdict_providers",
		"insert into policy_bundles",
		"insert into policy_detectors",
		"insert into policy_rules",
		"insert into sinks",
	} {
		if countQueries(tx.rowSQL, want) == 0 {
			t.Fatalf("row SQL calls did not include %q; calls=%s", want, joinedSQL(tx.rowSQL))
		}
	}
	for _, want := range []string{
		"insert into policy_rule_detectors",
		"insert into policy_rule_scopes",
		"insert into route_policy_bindings",
		"insert into route_limit_policies",
		"insert into client_route_limit_overrides",
		"insert into route_verdict_provider_bindings",
		"insert into retention_policies",
		"insert into raw_capture_policies",
	} {
		if countQueries(tx.execSQL, want) == 0 {
			t.Fatalf("exec SQL calls did not include %q; calls=%s", want, joinedSQL(tx.execSQL))
		}
	}
	if !hasExecArgs(tx.execArgs, "route-new", "verdict-new") {
		t.Fatalf("route verdict binding args = %#v, want remapped route and verdict provider IDs", tx.execArgs)
	}
	if !hasExecArgs(tx.execArgs, "rule-new", "detector-new") {
		t.Fatalf("policy rule detector args = %#v, want remapped rule and detector IDs", tx.execArgs)
	}
}

func TestEnsureDraftRevisionPropagatesCloneErrors(t *testing.T) {
	ctx := context.Background()
	tx := &fakeConfigTx{
		rowResults: []fakeConfigRow{
			rowWithConfigErr(pgx.ErrNoRows),
			rowWithConfigValues("active-1"),
			rowWithConfigValues("draft-1"),
		},
		rows: []*fakeConfigRows{
			{err: errCloneRowsFailed{}},
		},
	}

	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}
	err := repo.WithDraftRevision(ctx, "clone active revision", func(context.Context, revisionstore.Queryer, string) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "iterate providers for draft clone") {
		t.Fatalf("WithDraftRevision() error = %v, want provider clone iteration context", err)
	}
}

type errCloneRowsFailed struct{}

func (errCloneRowsFailed) Error() string { return "rows failed" }

func hasExecArgs(execArgs [][]any, want ...any) bool {
	for _, args := range execArgs {
		if containsAllArgs(args, want...) {
			return true
		}
	}
	return false
}

func containsAllArgs(args []any, want ...any) bool {
	for _, wanted := range want {
		found := false
		for _, arg := range args {
			if arg == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func joinedSQL(sqls []string) string {
	return strings.Join(sqls, "\n---\n")
}
