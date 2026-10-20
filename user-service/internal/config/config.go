package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Pool struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

type Database struct {
	PrimaryURL string
	ReplicaURL string

	Write Pool
	Read  Pool

	StatementTimeout time.Duration
	IdleInTxTimeout  time.Duration

	MaxReplicaLag    time.Duration
	LagCheckInterval time.Duration
}

type Config struct {
	LogLevel     slog.Level
	HTTPAddr     string
	GRPCAddr     string
	KafkaBrokers []string
	Database     Database

	ShutdownDelay   time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	r := &reader{}

	cfg := Config{
		LogLevel:        r.logLevel("LOG_LEVEL", slog.LevelInfo),
		HTTPAddr:        r.str("HTTP_ADDR", ":8082"),
		GRPCAddr:        r.str("GRPC_ADDR", ":9090"),
		KafkaBrokers:    r.list("KAFKA_BROKERS"),
		ShutdownDelay:   r.duration("SHUTDOWN_DELAY", 5*time.Second),
		ShutdownTimeout: r.duration("SHUTDOWN_TIMEOUT", 20*time.Second),
		Database: Database{
			PrimaryURL:       r.required("DATABASE_URL"),
			ReplicaURL:       r.str("DATABASE_REPLICA_URL", ""),
			Write:            r.pool("DB_WRITE", 10, 2),
			Read:             r.pool("DB_READ", 20, 2),
			StatementTimeout: r.duration("DB_STATEMENT_TIMEOUT", 10*time.Second),
			IdleInTxTimeout:  r.duration("DB_IDLE_IN_TX_TIMEOUT", 15*time.Second),
			MaxReplicaLag:    r.duration("DB_MAX_REPLICA_LAG", 5*time.Second),
			LagCheckInterval: r.duration("DB_LAG_CHECK_INTERVAL", 5*time.Second),
		},
	}

	for name, p := range map[string]Pool{"DB_WRITE": cfg.Database.Write, "DB_READ": cfg.Database.Read} {
		if p.MaxConns < 1 {
			r.errs = append(r.errs, fmt.Errorf("%s_MAX_CONNS must be >= 1", name))
		}
		if p.MinConns > p.MaxConns {
			r.errs = append(r.errs, fmt.Errorf("%s_MIN_CONNS must be <= %s_MAX_CONNS", name, name))
		}
	}

	if err := errors.Join(r.errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type reader struct {
	errs []error
}

func (r *reader) fail(format string, args ...any) {
	r.errs = append(r.errs, fmt.Errorf(format, args...))
}

func (r *reader) required(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		r.fail("%s is required", key)
	}
	return v
}

func (r *reader) str(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func (r *reader) i32(key string, def int32) int32 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n < 0 {
		r.fail("%s: invalid non-negative integer %q", key, v)
		return def
	}
	return int32(n)
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		r.fail("%s: invalid positive duration %q (example: 5s)", key, v)
		return def
	}
	return d
}

func (r *reader) list(key string) []string {
	var out []string
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		r.fail("%s is required (comma-separated host:port list)", key)
	}
	return out
}

func (r *reader) logLevel(key string, def slog.Level) slog.Level {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(v)); err != nil {
		r.fail("%s: invalid level %q (debug|info|warn|error)", key, v)
		return def
	}
	return l
}

func (r *reader) pool(prefix string, maxDef, minDef int32) Pool {
	return Pool{
		MaxConns:          r.i32(prefix+"_MAX_CONNS", maxDef),
		MinConns:          r.i32(prefix+"_MIN_CONNS", minDef),
		MaxConnLifetime:   r.duration(prefix+"_MAX_CONN_LIFETIME", 30*time.Minute),
		MaxConnIdleTime:   r.duration(prefix+"_MAX_CONN_IDLE_TIME", 5*time.Minute),
		HealthCheckPeriod: r.duration(prefix+"_HEALTH_CHECK_PERIOD", 30*time.Second),
	}
}
