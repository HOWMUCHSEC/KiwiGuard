package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateWritesDefaultsAndMapsDuplicates(t *testing.T) {
	q := &mutationQueryer{}
	err := Create(context.Background(), q, GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
		Notes:      "primary test client",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	mutationAssertArg(t, q.execArgs[0], 0, "client-acme")
	mutationAssertArg(t, q.execArgs[0], 2, "enabled")

	duplicate := &mutationQueryer{
		execErrs: []error{&pgconn.PgError{Code: "23505"}},
	}
	err = Create(context.Background(), duplicate, GatewayClient{ExternalID: "client-acme"})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Create() duplicate error = %v, want ErrAlreadyExists", err)
	}

	failed := &mutationQueryer{execErrs: []error{errors.New("insert failed")}}
	err = Create(context.Background(), failed, GatewayClient{ExternalID: "client-acme"})
	if err == nil {
		t.Fatal("Create() error = nil, want wrapped insert error")
	}
}

func TestUpsertPreservesRequestedStatus(t *testing.T) {
	revokedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	q := &mutationQueryer{}

	err := Upsert(context.Background(), q, GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "revoked",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
		Notes:      "primary test client",
		RevokedAt:  &revokedAt,
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	mutationAssertArg(t, q.execArgs[0], 0, "client-acme")
	mutationAssertArg(t, q.execArgs[0], 2, "revoked")
	mutationAssertArg(t, q.execArgs[0], 6, &revokedAt)
}

func TestUpsertReturnsWriteError(t *testing.T) {
	q := &mutationQueryer{execErrs: []error{errors.New("upsert failed")}}

	err := Upsert(context.Background(), q, GatewayClient{ExternalID: "client-acme"})
	if err == nil {
		t.Fatal("Upsert() error = nil, want wrapped write error")
	}
}

func TestRevokeReportsWhetherClientChanged(t *testing.T) {
	q := &mutationQueryer{commandTags: []pgconn.CommandTag{pgconn.NewCommandTag("UPDATE 1")}}

	changed, err := Revoke(context.Background(), q, "client-acme")
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if !changed {
		t.Fatal("Revoke() changed = false, want true")
	}
	mutationAssertArg(t, q.execArgs[0], 0, "client-acme")

	q = &mutationQueryer{commandTags: []pgconn.CommandTag{pgconn.NewCommandTag("UPDATE 0")}}
	changed, err = Revoke(context.Background(), q, "client-acme")
	if err != nil {
		t.Fatalf("Revoke() noop error = %v", err)
	}
	if changed {
		t.Fatal("Revoke() changed = true, want false")
	}
}

func TestRevokeReturnsWriteError(t *testing.T) {
	q := &mutationQueryer{execErrs: []error{errors.New("revoke failed")}}

	_, err := Revoke(context.Background(), q, "client-acme")
	if err == nil {
		t.Fatal("Revoke() error = nil, want wrapped write error")
	}
}

func TestBumpGenerationNotifiesWhenActiveRevisionExists(t *testing.T) {
	q := &mutationQueryer{
		rowsForQueryRow: []pgx.Row{mutationRowWithValues(int64(77))},
	}

	err := BumpGeneration(context.Background(), q, "kiwiguard_config_activated")
	if err != nil {
		t.Fatalf("BumpGeneration() error = %v", err)
	}
	if len(q.execArgs) != 2 {
		t.Fatalf("Exec calls = %d, want generation update and notification", len(q.execArgs))
	}
	mutationAssertArg(t, q.execArgs[1], 0, "kiwiguard_config_activated")
	mutationAssertArg(t, q.execArgs[1], 1, "77")
}

func TestBumpGenerationSkipsNotificationWithoutActiveRevision(t *testing.T) {
	q := &mutationQueryer{
		rowsForQueryRow: []pgx.Row{mutationRowWithErr(pgx.ErrNoRows)},
	}

	err := BumpGeneration(context.Background(), q, "kiwiguard_config_activated")
	if err != nil {
		t.Fatalf("BumpGeneration() error = %v", err)
	}
	if len(q.execArgs) != 1 {
		t.Fatalf("Exec calls = %d, want only generation update", len(q.execArgs))
	}
}

func TestBumpGenerationReturnsUpdateLoadAndNotifyErrors(t *testing.T) {
	tests := []struct {
		name string
		q    *mutationQueryer
	}{
		{
			name: "generation update",
			q:    &mutationQueryer{execErrs: []error{errors.New("update failed")}},
		},
		{
			name: "active revision load",
			q: &mutationQueryer{
				rowsForQueryRow: []pgx.Row{mutationRowWithErr(errors.New("load failed"))},
			},
		},
		{
			name: "notify",
			q: &mutationQueryer{
				rowsForQueryRow: []pgx.Row{mutationRowWithValues(int64(77))},
				execErrs:        []error{nil, errors.New("notify failed")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BumpGeneration(context.Background(), tt.q, "kiwiguard_config_activated")
			if err == nil {
				t.Fatal("BumpGeneration() error = nil, want error")
			}
		})
	}
}

func mutationAssertArg[T comparable](t *testing.T, args []any, index int, want T) {
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

type mutationQueryer struct {
	rowsForQueryRow []pgx.Row
	execArgs        [][]any
	execErrs        []error
	commandTags     []pgconn.CommandTag
}

func (q *mutationQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(q.rowsForQueryRow) == 0 {
		panic("unexpected QueryRow")
	}
	row := q.rowsForQueryRow[0]
	q.rowsForQueryRow = q.rowsForQueryRow[1:]
	return row
}

func (q *mutationQueryer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	q.execArgs = append(q.execArgs, args)
	if len(q.execErrs) > 0 {
		err := q.execErrs[0]
		q.execErrs = q.execErrs[1:]
		return pgconn.CommandTag{}, err
	}
	if len(q.commandTags) > 0 {
		tag := q.commandTags[0]
		q.commandTags = q.commandTags[1:]
		return tag, nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

type mutationFakeRow struct {
	values []any
	err    error
}

func mutationRowWithValues(values ...any) mutationFakeRow {
	return mutationFakeRow{values: values}
}

func mutationRowWithErr(err error) mutationFakeRow {
	return mutationFakeRow{err: err}
}

func (r mutationFakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan destination count does not match values")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("scan destination must be a non-nil pointer")
		}
		value := reflect.ValueOf(r.values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("scan value type is not assignable")
		}
		target.Elem().Set(value)
	}
	return nil
}
