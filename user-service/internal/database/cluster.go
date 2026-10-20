package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"user-service/internal/config"
)

type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Cluster struct {
	write *pgxpool.Pool
	read  *pgxpool.Pool
	cfg   config.Database
	log   *slog.Logger

	readHealthy atomic.Bool
	lagNanos    atomic.Int64
}

func NewCluster(ctx context.Context, cfg config.Database, log *slog.Logger) (*Cluster, error) {
	write, err := newPool(cfg.PrimaryURL, cfg.Write, cfg, "write", true)
	if err != nil {
		return nil, fmt.Errorf("primary pool: %w", err)
	}

	if err := write.Ping(ctx); err != nil {
		write.Close()
		return nil, fmt.Errorf("ping primary: %w", err)
	}

	c := &Cluster{write: write, cfg: cfg, log: log.With("component", "db")}

	if cfg.ReplicaURL == "" {
		c.log.Info("no replica configured, reads go to primary")
		return c, nil
	}

	read, err := newPool(cfg.ReplicaURL, cfg.Read, cfg, "read", false)
	if err != nil {
		write.Close()
		return nil, fmt.Errorf("replica pool: %w", err)
	}

	c.read = read
	c.check(ctx)
	return c, nil
}

func newPool(dsn string, p config.Pool, db config.Database, name string, requireWritable bool) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	pc.MaxConns = p.MaxConns
	pc.MinConns = p.MinConns
	pc.MaxConnLifetime = p.MaxConnLifetime
	pc.MaxConnLifetimeJitter = p.MaxConnLifetime / 10
	pc.MaxConnIdleTime = p.MaxConnIdleTime
	pc.HealthCheckPeriod = p.HealthCheckPeriod
	pc.ConnConfig.ConnectTimeout = 5 * time.Second

	rp := pc.ConnConfig.RuntimeParams
	rp["application_name"] = "user-service-" + name
	rp["statement_timeout"] = strconv.FormatInt(db.StatementTimeout.Milliseconds(), 10)
	rp["idle_in_transaction_session_timeout"] = strconv.FormatInt(db.IdleInTxTimeout.Milliseconds(), 10)

	if requireWritable {
		pc.ConnConfig.ValidateConnect = pgconn.ValidateConnectTargetSessionAttrsReadWrite
	}

	return pgxpool.NewWithConfig(context.Background(), pc)
}

func (c *Cluster) Writer() Querier { return c.write }

func (c *Cluster) Reader() Querier {
	if c.read != nil && c.readHealthy.Load() {
		return c.read
	}
	return c.write
}

func (c *Cluster) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, c.write, pgx.TxOptions{}, fn)
}

func Read[T any](ctx context.Context, c *Cluster, fn func(q Querier) (T, error)) (T, error) {
	if c.read == nil || !c.readHealthy.Load() {
		return fn(c.write)
	}
	v, err := fn(c.read)
	if err != nil && ctx.Err() == nil && shouldFallback(err) {
		c.log.WarnContext(ctx, "replica read failed, retrying on primary", "err", err)
		return fn(c.write)
	}

	return v, err
}

func shouldFallback(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case strings.Contains(pgErr.Message, "conflict with recovery"): // SQLSTATE 40001 / 57014
			return true
		case strings.HasPrefix(pgErr.Code, "08"): // connection exception
			return true
		case strings.HasPrefix(pgErr.Code, "57P"): // admin/crash shutdown, cannot connect now
			return true
		}
		return false // real SQL errors: the primary would fail the same way
	}
	return true // network errors, pool acquire timeouts, etc.
}

// Monitor polls the replica until ctx is cancelled and flips it in/out of rotation.
func (c *Cluster) Monitor(ctx context.Context) {
	if c.read == nil {
		return
	}
	t := time.NewTicker(c.cfg.LagCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.check(ctx)
		}
	}
}

// When the primary is idle, pg_last_xact_replay_timestamp() stops advancing and
// naive "now() - replay_ts" would report fake lag. If everything received has
// been replayed, the replica is caught up, so lag is 0.
const lagQuery = `
SELECT CASE
  WHEN NOT pg_is_in_recovery() THEN 0
  WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 0
  ELSE COALESCE(EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp()), 0)
END::float8`

func (c *Cluster) check(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var lagSec float64
	if err := c.read.QueryRow(cctx, lagQuery).Scan(&lagSec); err != nil {
		if ctx.Err() == nil {
			c.setHealthy(false, "replica check failed", "err", err)
		}
		return
	}

	lag := time.Duration(lagSec * float64(time.Second))
	c.lagNanos.Store(int64(lag))
	if lag > c.cfg.MaxReplicaLag {
		c.setHealthy(false, "replica lag too high", "lag", lag, "max", c.cfg.MaxReplicaLag)
		return
	}
	c.setHealthy(true, "replica healthy", "lag", lag)
}

// setHealthy logs only on state transitions, so a dead replica doesn't flood the logs.
func (c *Cluster) setHealthy(healthy bool, msg string, attrs ...any) {
	if prev := c.readHealthy.Swap(healthy); prev != healthy {
		if healthy {
			c.log.Info(msg, attrs...)
		} else {
			c.log.Warn(msg+", reads fall back to primary", attrs...)
		}
	}
}

// Ping checks the primary; the service cannot work without it.
func (c *Cluster) Ping(ctx context.Context) error { return c.write.Ping(ctx) }

type ReplicaStatus struct {
	Configured bool
	Healthy    bool
	Lag        time.Duration
}

func (c *Cluster) ReplicaStatus() ReplicaStatus {
	return ReplicaStatus{
		Configured: c.read != nil,
		Healthy:    c.read != nil && c.readHealthy.Load(),
		Lag:        time.Duration(c.lagNanos.Load()),
	}
}

func (c *Cluster) Close() {
	c.write.Close()
	if c.read != nil {
		c.read.Close()
	}
}
