package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrAlreadyExists is returned when creating a duplicate gateway client.
	ErrAlreadyExists = errors.New("gateway client already exists")
)

// Create writes a new gateway client and fails when it already exists.
func Create(ctx context.Context, q Mutator, client GatewayClient) error {
	if client.Status == "" {
		client.Status = "enabled"
	}
	_, err := q.Exec(ctx, `
		insert into gateway_clients (external_id, name, status, key_prefix, key_hash, notes, revoked_at)
		values ($1, $2, $3, $4, $5, $6, coalesce($7, case when $3 = 'revoked' then now() end))
	`, client.ExternalID, client.Name, client.Status, client.KeyPrefix, client.KeyHash, client.Notes, client.RevokedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("create gateway client: %w", err)
	}
	return nil
}

// Upsert writes a gateway client, preserving revoked clients as revoked.
func Upsert(ctx context.Context, q Mutator, client GatewayClient) error {
	if client.Status == "" {
		client.Status = "enabled"
	}
	_, err := q.Exec(ctx, `
		insert into gateway_clients (external_id, name, status, key_prefix, key_hash, notes, revoked_at)
		values ($1, $2, $3, $4, $5, $6, coalesce($7, case when $3 = 'revoked' then now() end))
		on conflict (external_id) do update
		set name = excluded.name,
			status = case
				when gateway_clients.status = 'revoked' then gateway_clients.status
				else excluded.status
			end,
			key_prefix = excluded.key_prefix,
			key_hash = excluded.key_hash,
			notes = excluded.notes,
			updated_at = now(),
			revoked_at = case
				when gateway_clients.status = 'revoked' then gateway_clients.revoked_at
				when excluded.status = 'revoked' then coalesce(excluded.revoked_at, now())
				else excluded.revoked_at
			end
	`, client.ExternalID, client.Name, client.Status, client.KeyPrefix, client.KeyHash, client.Notes, client.RevokedAt)
	if err != nil {
		return fmt.Errorf("upsert gateway client: %w", err)
	}
	return nil
}

// Revoke marks a gateway client as revoked and reports whether a row changed.
func Revoke(ctx context.Context, q Mutator, clientID string) (bool, error) {
	commandTag, err := q.Exec(ctx, `
		update gateway_clients
		set status = 'revoked',
			updated_at = now(),
			revoked_at = coalesce(revoked_at, now())
		where (id::text = $1 or external_id = $1) and status <> 'revoked'
	`, clientID)
	if err != nil {
		return false, fmt.Errorf("revoke gateway client: %w", err)
	}
	return commandTag.RowsAffected() > 0, nil
}

// BumpGeneration advances the gateway-client runtime token and notifies active runtime watchers.
func BumpGeneration(ctx context.Context, q Mutator, channel string) error {
	if _, err := q.Exec(ctx, `
		update gateway_client_config_versions
		set generation = generation + 1,
			updated_at = now()
		where id
	`); err != nil {
		return fmt.Errorf("bump gateway client generation: %w", err)
	}
	revisionNumber, err := activeRevisionNumber(ctx, q)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `select pg_notify($1, $2)`, channel, strconv.FormatInt(revisionNumber, 10)); err != nil {
		return fmt.Errorf("notify gateway client generation: %w", err)
	}
	return nil
}

func activeRevisionNumber(ctx context.Context, q Mutator) (int64, error) {
	var number int64
	err := q.QueryRow(ctx, `
		select cr.revision_number + coalesce(gv.generation, 0)
		from config_revisions cr
		cross join gateway_client_config_versions gv
		where cr.status = 'active'
		order by cr.revision_number desc
		limit 1
	`).Scan(&number)
	if err != nil {
		return 0, fmt.Errorf("load active revision number: %w", err)
	}
	return number, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
