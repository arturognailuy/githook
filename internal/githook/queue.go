package githook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Job struct {
	DeliveryID string
	RunID      int64
	HeadSHA    string
	Attempts   int
}
type Queue struct{ db *sql.DB }

func OpenQueue(path string) (*Queue, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	q := &Queue{db: db}
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS jobs (delivery_id TEXT PRIMARY KEY, run_id INTEGER NOT NULL UNIQUE, head_sha TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'queued', attempts INTEGER NOT NULL DEFAULT 0, available_at INTEGER NOT NULL, created_at INTEGER NOT NULL, last_error TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS jobs_claim ON jobs(status, available_at, run_id);
CREATE TABLE IF NOT EXISTS deployment_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), run_id INTEGER NOT NULL, head_sha TEXT NOT NULL, deployed_at INTEGER NOT NULL);`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize queue: %w", err)
	}
	return q, nil
}
func (q *Queue) Close() error { return q.db.Close() }
func (q *Queue) Enqueue(ctx context.Context, deliveryID string, runID int64, headSHA string) (bool, error) {
	now := time.Now().Unix()
	r, err := q.db.ExecContext(ctx, `INSERT OR IGNORE INTO jobs(delivery_id,run_id,head_sha,available_at,created_at) VALUES(?,?,?,?,?)`, deliveryID, runID, headSHA, now, now)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (q *Queue) Claim(ctx context.Context) (Job, error) {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	var j Job
	err = tx.QueryRowContext(ctx, `SELECT delivery_id,run_id,head_sha,attempts FROM jobs WHERE status='queued' AND available_at<=? ORDER BY run_id DESC LIMIT 1`, time.Now().Unix()).Scan(&j.DeliveryID, &j.RunID, &j.HeadSHA, &j.Attempts)
	if err != nil {
		return Job{}, err
	}
	r, err := tx.ExecContext(ctx, `UPDATE jobs SET status='processing',attempts=attempts+1 WHERE delivery_id=? AND status='queued'`, j.DeliveryID)
	if err != nil {
		return Job{}, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return Job{}, errors.New("job was claimed concurrently")
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	j.Attempts++
	return j, nil
}
func (q *Queue) Complete(ctx context.Context, deliveryID string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE jobs SET status='done',last_error='' WHERE delivery_id=?`, deliveryID)
	return err
}
func (q *Queue) Retry(ctx context.Context, deliveryID, message string, delay time.Duration) error {
	_, err := q.db.ExecContext(ctx, `UPDATE jobs SET status='queued',last_error=?,available_at=? WHERE delivery_id=?`, message, time.Now().Add(delay).Unix(), deliveryID)
	return err
}
func (q *Queue) Recover(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, `UPDATE jobs SET status='queued',available_at=? WHERE status='processing'`, time.Now().Unix())
	return err
}
func (q *Queue) RefuseOlder(ctx context.Context, runID int64) (bool, error) {
	var current int64
	err := q.db.QueryRowContext(ctx, `SELECT run_id FROM deployment_state WHERE singleton=1`).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return runID <= current, err
}
func (q *Queue) MarkDeployed(ctx context.Context, runID int64, headSHA string) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO deployment_state(singleton,run_id,head_sha,deployed_at) VALUES(1,?,?,?) ON CONFLICT(singleton) DO UPDATE SET run_id=excluded.run_id,head_sha=excluded.head_sha,deployed_at=excluded.deployed_at WHERE excluded.run_id>deployment_state.run_id`, runID, headSHA, time.Now().Unix())
	return err
}
