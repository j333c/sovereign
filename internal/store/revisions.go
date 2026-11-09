package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"strconv"
)

// ComposeRevision tracks declarative Compose revisions and snapshots.
type ComposeRevision struct {
	Revision           int    `json:"revision"`
	ServiceID          string `json:"service_id"`
	OldImage           string `json:"old_image"`
	NewImage           string `json:"new_image"`
	ComposeFilePath    string `json:"compose_file_path"`
	SQLiteSnapshotPath string `json:"sqlite_snapshot_path"`
	VolumeBackupPath   string `json:"volume_backup_path"`
	Status             string `json:"status"`
	CreatedAt          string `json:"created_at"`
}

// GetNextRevisionNumber returns the next sequential revision number.
func (d *DB) GetNextRevisionNumber(ctx context.Context) (int, error) {
	var nextRev int
	err := d.QueryRowContext(ctx, "SELECT COALESCE(MAX(revision), 0) + 1 FROM compose_revisions").Scan(&nextRev)
	if err != nil {
		return 0, fmt.Errorf("failed to query next revision number: %w", err)
	}
	return nextRev, nil
}

// RecordRevision inserts a revision record into compose_revisions and updates active_compose_revision.
func (d *DB) RecordRevision(ctx context.Context, rev *ComposeRevision) error {
	if rev == nil {
		return errors.New("compose revision cannot be nil")
	}
	if rev.Status == "" {
		rev.Status = "active"
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin revision transaction: %w", err)
	}
	defer tx.Rollback()

	if rev.Revision > 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO compose_revisions (revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, rev.Revision, rev.ServiceID, rev.OldImage, rev.NewImage, rev.ComposeFilePath, rev.SQLiteSnapshotPath, rev.VolumeBackupPath, rev.Status)
		if err != nil {
			return fmt.Errorf("failed to insert compose revision: %w", err)
		}
	} else {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO compose_revisions (service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, rev.ServiceID, rev.OldImage, rev.NewImage, rev.ComposeFilePath, rev.SQLiteSnapshotPath, rev.VolumeBackupPath, rev.Status)
		if err != nil {
			return fmt.Errorf("failed to insert compose revision: %w", err)
		}
		id, err := res.LastInsertId()
		if err == nil {
			rev.Revision = int(id)
		}
	}

	if rev.Status == "active" {
		// Supersede previous active revisions
		_, err := tx.ExecContext(ctx, `
			UPDATE compose_revisions
			SET status = 'superseded'
			WHERE status = 'active' AND revision != ?
		`, rev.Revision)
		if err != nil {
			return fmt.Errorf("failed to supersede prior active revisions: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO system_state (key, value, updated_at)
			VALUES ('active_compose_revision', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
		`, strconv.Itoa(rev.Revision))
		if err != nil {
			return fmt.Errorf("failed to set active revision in system state: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit revision transaction: %w", err)
	}

	// Fetch recorded timestamp
	_ = d.QueryRowContext(ctx, "SELECT created_at FROM compose_revisions WHERE revision = ?", rev.Revision).Scan(&rev.CreatedAt)
	return nil
}

// GetRevision fetches a compose revision by its revision number.
func (d *DB) GetRevision(ctx context.Context, revision int) (*ComposeRevision, error) {
	row := d.QueryRowContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		WHERE revision = ?
	`, revision)

	var rev ComposeRevision
	if err := row.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query compose revision %d: %w", revision, err)
	}
	return &rev, nil
}

// GetActiveRevision returns the currently active compose revision.
func (d *DB) GetActiveRevision(ctx context.Context) (*ComposeRevision, error) {
	activeRevNum, err := d.GetActiveRevisionNumber(ctx)
	if err == nil && activeRevNum > 0 {
		rev, err := d.GetRevision(ctx, activeRevNum)
		if err == nil && rev.Status == "active" {
			return rev, nil
		}
	}

	row := d.QueryRowContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		WHERE status = 'active'
		ORDER BY revision DESC
		LIMIT 1
	`)

	var rev ComposeRevision
	if err := row.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to query active compose revision: %w", err)
	}
	return &rev, nil
}

// ListRevisions returns all compose revisions ordered by revision descending.
func (d *DB) ListRevisions(ctx context.Context) ([]ComposeRevision, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT revision, service_id, old_image, new_image, compose_file_path, sqlite_snapshot_path, volume_backup_path, status, created_at
		FROM compose_revisions
		ORDER BY revision DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list compose revisions: %w", err)
	}
	defer rows.Close()

	revisions := make([]ComposeRevision, 0)
	for rows.Next() {
		var rev ComposeRevision
		if err := rows.Scan(&rev.Revision, &rev.ServiceID, &rev.OldImage, &rev.NewImage, &rev.ComposeFilePath, &rev.SQLiteSnapshotPath, &rev.VolumeBackupPath, &rev.Status, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan compose revision: %w", err)
		}
		revisions = append(revisions, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating compose revisions: %w", err)
	}
	return revisions, nil
}

// UpdateRevisionStatus updates the status of a specific revision.
func (d *DB) UpdateRevisionStatus(ctx context.Context, revision int, status string) error {
	res, err := d.ExecContext(ctx, `
		UPDATE compose_revisions
		SET status = ?
		WHERE revision = ?
	`, status, revision)
	if err != nil {
		return fmt.Errorf("failed to update revision %d status: %w", revision, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if status == "active" {
		_, _ = d.ExecContext(ctx, `
			UPDATE compose_revisions
			SET status = 'superseded'
			WHERE status = 'active' AND revision != ?
		`, revision)
		_ = d.SetActiveRevisionNumber(ctx, revision)
	} else {
		activeRevNum, err := d.GetActiveRevisionNumber(ctx)
		if err == nil && activeRevNum == revision {
			var latestActive int
			err := d.QueryRowContext(ctx, "SELECT revision FROM compose_revisions WHERE status = 'active' ORDER BY revision DESC LIMIT 1").Scan(&latestActive)
			if err == nil && latestActive > 0 {
				_ = d.SetActiveRevisionNumber(ctx, latestActive)
			} else {
				_, _ = d.ExecContext(ctx, "DELETE FROM system_state WHERE key = 'active_compose_revision'")
			}
		}
	}
	return nil
}

// SetActiveRevisionNumber records the active revision number in system_state.
func (d *DB) SetActiveRevisionNumber(ctx context.Context, revision int) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO system_state (key, value, updated_at)
		VALUES ('active_compose_revision', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, strconv.Itoa(revision))
	if err != nil {
		return fmt.Errorf("failed to set active_compose_revision in system_state: %w", err)
	}
	return nil
}

// GetActiveRevisionNumber reads the active revision number from system_state.
func (d *DB) GetActiveRevisionNumber(ctx context.Context) (int, error) {
	var val string
	err := d.QueryRowContext(ctx, "SELECT value FROM system_state WHERE key = 'active_compose_revision'").Scan(&val)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to query active_compose_revision: %w", err)
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return 0, nil
	}
	return num, nil
}
