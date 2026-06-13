package revisionstore

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestActiveRevisionNumberLoadsGenerationAdjustedRevision(t *testing.T) {
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithValues(int64(42))},
	}

	got, err := activeRevisionNumber(context.Background(), q)
	if err != nil {
		t.Fatalf("activeRevisionNumber() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("activeRevisionNumber() = %d, want 42", got)
	}
}

func TestActiveRevisionNumberMapsMissingActiveRevision(t *testing.T) {
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithErr(pgx.ErrNoRows)},
	}

	_, err := activeRevisionNumber(context.Background(), q)
	if !errors.Is(err, ErrActiveRevisionNotFound) {
		t.Fatalf("activeRevisionNumber() error = %v, want ErrActiveRevisionNotFound", err)
	}
}

func TestActiveRevisionNumberWrapsQueryError(t *testing.T) {
	errBoom := errors.New("database unavailable")
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithErr(errBoom)},
	}

	_, err := activeRevisionNumber(context.Background(), q)
	if !errors.Is(err, errBoom) {
		t.Fatalf("activeRevisionNumber() error = %v, want wrapped query error", err)
	}
}

func TestActiveRevisionScansRuntimeRevisionMetadata(t *testing.T) {
	activatedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithValues(
			"revision-id",
			int64(43),
			"control",
			"active",
			"alice",
			"snapshot-hash",
			"snapshot-ref",
			activatedAt,
		)},
	}

	got, err := activeRevision(context.Background(), q)
	if err != nil {
		t.Fatalf("activeRevision() error = %v", err)
	}

	want := ConfigRevision{
		ID:                   "revision-id",
		Number:               43,
		Source:               "control",
		Status:               "active",
		Actor:                "alice",
		CompiledSnapshotHash: "snapshot-hash",
		CompiledSnapshotRef:  "snapshot-ref",
		ActivatedAt:          activatedAt,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("activeRevision() = %#v, want %#v", got, want)
	}
}

func TestActiveRevisionMapsMissingActiveRevision(t *testing.T) {
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithErr(pgx.ErrNoRows)},
	}

	_, err := activeRevision(context.Background(), q)
	if !errors.Is(err, ErrActiveRevisionNotFound) {
		t.Fatalf("activeRevision() error = %v, want ErrActiveRevisionNotFound", err)
	}
}

func TestLatestDraftRevisionIDLoadsDraft(t *testing.T) {
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithValues("draft-1")},
	}

	got, err := latestDraftRevisionID(context.Background(), q)
	if err != nil {
		t.Fatalf("latestDraftRevisionID() error = %v", err)
	}
	if got != "draft-1" {
		t.Fatalf("latestDraftRevisionID() = %q, want draft-1", got)
	}
}

func TestLatestDraftRevisionIDMapsMissingDraft(t *testing.T) {
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithErr(pgx.ErrNoRows)},
	}

	_, err := latestDraftRevisionID(context.Background(), q)
	if !errors.Is(err, ErrActiveRevisionNotFound) {
		t.Fatalf("latestDraftRevisionID() error = %v, want ErrActiveRevisionNotFound", err)
	}
}

func TestLatestDraftRevisionIDWrapsQueryError(t *testing.T) {
	errBoom := errors.New("database unavailable")
	q := &coreRecordingQueryer{
		rowsForQueryRow: []pgx.Row{coreRowWithErr(errBoom)},
	}

	_, err := latestDraftRevisionID(context.Background(), q)
	if !errors.Is(err, errBoom) {
		t.Fatalf("latestDraftRevisionID() error = %v, want wrapped query error", err)
	}
}

func TestCurrentRevisionIDMapsMissingRevision(t *testing.T) {
	unit := NewUnitOfWork(&coreRecordingQueryer{rowsForQueryRow: []pgx.Row{coreRowWithErr(pgx.ErrNoRows)}}, Options{})

	_, err := unit.CurrentRevisionID(context.Background())
	if !errors.Is(err, ErrActiveRevisionNotFound) {
		t.Fatalf("CurrentRevisionID() error = %v, want ErrActiveRevisionNotFound", err)
	}
}

func TestCurrentRevisionIDLoadsCurrentRevision(t *testing.T) {
	unit := NewUnitOfWork(&coreRecordingQueryer{rowsForQueryRow: []pgx.Row{coreRowWithValues("revision-1")}}, Options{})

	got, err := unit.CurrentRevisionID(context.Background())
	if err != nil {
		t.Fatalf("CurrentRevisionID() error = %v", err)
	}
	if got != "revision-1" {
		t.Fatalf("CurrentRevisionID() = %q, want revision-1", got)
	}
}

func TestCurrentRevisionIDWrapsQueryError(t *testing.T) {
	errBoom := errors.New("database unavailable")
	unit := NewUnitOfWork(&coreRecordingQueryer{rowsForQueryRow: []pgx.Row{coreRowWithErr(errBoom)}}, Options{})

	_, err := unit.CurrentRevisionID(context.Background())
	if !errors.Is(err, errBoom) {
		t.Fatalf("CurrentRevisionID() error = %v, want wrapped query error", err)
	}
}

type coreRecordingQueryer struct {
	rowsForQueryRow []pgx.Row
}

func (q *coreRecordingQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (q *coreRecordingQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(q.rowsForQueryRow) == 0 {
		panic("unexpected QueryRow")
	}
	row := q.rowsForQueryRow[0]
	q.rowsForQueryRow = q.rowsForQueryRow[1:]
	return row
}

func (q *coreRecordingQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (q *coreRecordingQueryer) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	panic("unexpected BeginTx")
}

type coreFakeRow struct {
	values []any
	err    error
}

func coreRowWithValues(values ...any) coreFakeRow {
	return coreFakeRow{values: values}
}

func coreRowWithErr(err error) coreFakeRow {
	return coreFakeRow{err: err}
}

func (r coreFakeRow) Scan(dest ...any) error {
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
