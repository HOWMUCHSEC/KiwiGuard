package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestActivatePolicyBundlesCommitsActivationTransaction(t *testing.T) {
	ctx := context.Background()
	tx := &fakeConfigTx{
		rows: []*fakeConfigRows{
			newConfigRows([][]any{{"bundle-1", "pii", "builtin", "v1", "PII bundle", "allow", false, json.RawMessage(`{"kind":"test"}`)}}),
			newConfigRows(nil),
			newConfigRows(nil),
		},
		rowResults: []fakeConfigRow{
			rowWithConfigValues("draft-1"),
			rowWithConfigValues("snapshot-1"),
			rowWithConfigValues(int64(10)),
		},
	}
	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}

	result, err := testActivatePolicyBundles(ctx, repo, policystore.ActivationRequest{
		Keys:         []string{"pii"},
		SnapshotHash: "sha256:compiled",
		Reason:       "smoke test activation",
	})
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	if result.RevisionNumber != 10 || result.SnapshotHash != "sha256:compiled" || !reflect.DeepEqual(result.ActiveKeys, []string{"pii"}) {
		t.Fatalf("ActivatePolicyBundles() = %+v, want revision 10, hash, active key", result)
	}
	if !tx.committed {
		t.Fatal("ActivatePolicyBundles() did not commit transaction")
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollback calls = %d, want deferred rollback after commit", tx.rollbacks)
	}
	if countQueries(tx.execSQL, "policy_activation_records") != 1 {
		t.Fatalf("policy activation record writes = %d, want 1", countQueries(tx.execSQL, "policy_activation_records"))
	}
	if got := tx.execArgs[0]; !reflect.DeepEqual(got, []any{"draft-1", []string{"pii"}}) {
		t.Fatalf("activation update args = %#v, want draft id and keys", got)
	}
	if got := tx.execArgs[2]; !reflect.DeepEqual(got, []any{"draft-1", "system", "sha256:compiled"}) {
		t.Fatalf("revision activation args = %#v, want default system actor", got)
	}
}

func TestActivatePolicyBundlesRollsBackWhenSnapshotHashMissing(t *testing.T) {
	ctx := context.Background()
	tx := &fakeConfigTx{
		rows: []*fakeConfigRows{
			newConfigRows([][]any{{"bundle-1", "pii", "builtin", "v1", "PII bundle", "allow", false, json.RawMessage(`{}`)}}),
			newConfigRows(nil),
			newConfigRows(nil),
		},
		rowResults: []fakeConfigRow{
			rowWithConfigValues("draft-1"),
		},
	}
	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}

	_, err := testActivatePolicyBundles(ctx, repo, policystore.ActivationRequest{Keys: []string{"pii"}})
	if err == nil || !strings.Contains(err.Error(), "snapshot hash is required") {
		t.Fatalf("ActivatePolicyBundles() error = %v, want snapshot hash validation", err)
	}
	if tx.committed {
		t.Fatal("ActivatePolicyBundles() committed a validation failure")
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollback calls = %d, want 1", tx.rollbacks)
	}
	if len(tx.execSQL) != 0 {
		t.Fatalf("exec calls = %d, want no writes after validation failure", len(tx.execSQL))
	}
}

func TestConfigRepositoryActiveRevisionNumberUsesRepositoryDB(t *testing.T) {
	ctx := context.Background()
	db := &fakeConfigDB{rowResults: []fakeConfigRow{rowWithConfigValues(int64(27))}}
	repo := &ConfigRepository{pool: db}

	number, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() error = %v", err)
	}
	if number != 27 {
		t.Fatalf("ActiveRevisionNumber() = %d, want 27", number)
	}
	if len(db.rowSQL) != 1 || !strings.Contains(db.rowSQL[0], "gateway_client_config_versions") {
		t.Fatalf("ActiveRevisionNumber() SQL calls = %#v, want active revision query", db.rowSQL)
	}
}

type fakeConfigDB struct {
	tx         *fakeConfigTx
	beginErr   error
	rows       []*fakeConfigRows
	rowResults []fakeConfigRow
	rowSQL     []string
}

func (db *fakeConfigDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	if db.beginErr != nil {
		return nil, db.beginErr
	}
	return db.tx, nil
}

func (db *fakeConfigDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if len(db.rows) == 0 {
		return nil, errors.New("unexpected query")
	}
	rows := db.rows[0]
	db.rows = db.rows[1:]
	return rows, nil
}

func (db *fakeConfigDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	db.rowSQL = append(db.rowSQL, sql)
	if len(db.rowResults) == 0 {
		return rowWithConfigErr(errors.New("unexpected query row"))
	}
	row := db.rowResults[0]
	db.rowResults = db.rowResults[1:]
	return row
}

func (db *fakeConfigDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

type fakeConfigTx struct {
	rows       []*fakeConfigRows
	rowResults []fakeConfigRow
	commitErr  error
	querySQL   []string
	rowSQL     []string
	rowArgs    [][]any
	execSQL    []string
	execArgs   [][]any
	committed  bool
	rollbacks  int
}

func (tx *fakeConfigTx) Begin(context.Context) (pgx.Tx, error) {
	return tx, nil
}

func (tx *fakeConfigTx) Commit(context.Context) error {
	tx.committed = true
	return tx.commitErr
}

func (tx *fakeConfigTx) Rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

func (tx *fakeConfigTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected copy from")
}

func (tx *fakeConfigTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeConfigTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeConfigTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected prepare")
}

func (tx *fakeConfigTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.execSQL = append(tx.execSQL, sql)
	tx.execArgs = append(tx.execArgs, append([]any(nil), args...))
	return pgconn.CommandTag{}, nil
}

func (tx *fakeConfigTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	tx.querySQL = append(tx.querySQL, sql)
	if len(tx.rows) == 0 {
		return nil, fmt.Errorf("unexpected query: %s", sql)
	}
	rows := tx.rows[0]
	tx.rows = tx.rows[1:]
	return rows, nil
}

func (tx *fakeConfigTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	tx.rowSQL = append(tx.rowSQL, sql)
	tx.rowArgs = append(tx.rowArgs, append([]any(nil), args...))
	if len(tx.rowResults) == 0 {
		return rowWithConfigErr(fmt.Errorf("unexpected query row: %s", sql))
	}
	row := tx.rowResults[0]
	tx.rowResults = tx.rowResults[1:]
	return row
}

func (tx *fakeConfigTx) Conn() *pgx.Conn {
	return nil
}

type fakeConfigRows struct {
	values [][]any
	index  int
	err    error
}

func newConfigRows(values [][]any) *fakeConfigRows {
	return &fakeConfigRows{values: values, index: -1}
}

func (r *fakeConfigRows) Close() {}

func (r *fakeConfigRows) Err() error {
	return r.err
}

func (r *fakeConfigRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeConfigRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeConfigRows) Next() bool {
	if r.index+1 >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *fakeConfigRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	return assignConfigValues(dest, r.values[r.index])
}

func (r *fakeConfigRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.values) {
		return nil, errors.New("values without current row")
	}
	return r.values[r.index], nil
}

func (r *fakeConfigRows) RawValues() [][]byte {
	return nil
}

func (r *fakeConfigRows) Conn() *pgx.Conn {
	return nil
}

type fakeConfigRow struct {
	values []any
	err    error
}

func rowWithConfigValues(values ...any) fakeConfigRow {
	return fakeConfigRow{values: values}
}

func rowWithConfigErr(err error) fakeConfigRow {
	return fakeConfigRow{err: err}
}

func (r fakeConfigRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignConfigValues(dest, r.values)
}

func assignConfigValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan target count %d does not match value count %d", len(dest), len(values))
	}
	for i := range values {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return fmt.Errorf("scan target %d is not a pointer", i)
		}
		value := reflect.ValueOf(values[i])
		if !value.IsValid() {
			target.Elem().Set(reflect.Zero(target.Elem().Type()))
			continue
		}
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
			continue
		}
		if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
			continue
		}
		return fmt.Errorf("cannot assign %T to %T", values[i], dest[i])
	}
	return nil
}

func countQueries(sqls []string, needle string) int {
	var count int
	for _, sql := range sqls {
		if strings.Contains(sql, needle) {
			count++
		}
	}
	return count
}
