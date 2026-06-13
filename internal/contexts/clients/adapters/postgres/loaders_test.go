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

func TestLoadGatewayClientsScansRows(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	revokedAt := updatedAt.Add(time.Hour)
	q := &recordingQueryer{
		rows: newRows([][]any{
			{
				"client-id",
				"client-acme",
				"Acme Console",
				"revoked",
				"kg_acme",
				"sha256:acme",
				"primary test client",
				createdAt,
				updatedAt,
				&revokedAt,
			},
		}),
	}

	got, err := LoadGatewayClients(context.Background(), q)
	if err != nil {
		t.Fatalf("LoadGatewayClients() error = %v", err)
	}

	want := []GatewayClient{{
		ID:         "client-id",
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "revoked",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
		Notes:      "primary test client",
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		RevokedAt:  &revokedAt,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadGatewayClients() = %#v, want %#v", got, want)
	}
}

func TestLoadGatewayClientsReturnsEmptySlice(t *testing.T) {
	q := &recordingQueryer{rows: newRows(nil)}

	got, err := LoadGatewayClients(context.Background(), q)
	if err != nil {
		t.Fatalf("LoadGatewayClients() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadGatewayClients() length = %d, want 0", len(got))
	}
}

func TestLoadGatewayClientsReturnsQueryScanAndIterationErrors(t *testing.T) {
	queryErr := errors.New("query failed")
	scanErr := errors.New("scan failed")
	iterateErr := errors.New("iterate failed")

	tests := []struct {
		name string
		q    *recordingQueryer
	}{
		{
			name: "query",
			q:    &recordingQueryer{queryErr: queryErr},
		},
		{
			name: "scan",
			q: &recordingQueryer{
				rows: &fakeRows{
					values:  [][]any{{"client-id"}},
					index:   -1,
					scanErr: scanErr,
				},
			},
		},
		{
			name: "iterate",
			q: &recordingQueryer{
				rows: &fakeRows{err: iterateErr, index: -1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadGatewayClients(context.Background(), tt.q)
			if err == nil {
				t.Fatal("LoadGatewayClients() error = nil, want error")
			}
		})
	}
}

type recordingQueryer struct {
	rows     pgx.Rows
	queryErr error
}

func (q *recordingQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	if q.rows != nil {
		return q.rows, nil
	}
	panic("unexpected Query")
}

type fakeRows struct {
	values  [][]any
	index   int
	err     error
	scanErr error
}

func newRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
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
	if r.scanErr != nil {
		return r.scanErr
	}
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
