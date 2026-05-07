package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestInfra struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	pgC    testcontainers.Container
	redisC testcontainers.Container
}

func SetupContainers(ctx context.Context) (*TestInfra, error) {
	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}

	pgConnStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgC.Terminate(ctx)
		return nil, fmt.Errorf("postgres connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, pgConnStr)
	if err != nil {
		pgC.Terminate(ctx)
		return nil, fmt.Errorf("pgxpool: %w", err)
	}

	redisC, err := tcredis.Run(ctx, "redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		pool.Close()
		pgC.Terminate(ctx)
		return nil, fmt.Errorf("start redis: %w", err)
	}

	redisEndpoint, err := redisC.Endpoint(ctx, "")
	if err != nil {
		pool.Close()
		pgC.Terminate(ctx)
		redisC.Terminate(ctx)
		return nil, fmt.Errorf("redis endpoint: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisEndpoint})

	return &TestInfra{
		Pool:   pool,
		Redis:  rdb,
		pgC:    pgC,
		redisC: redisC,
	}, nil
}

func (ti *TestInfra) RunMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			if len(e.Name()) > 7 && e.Name()[len(e.Name())-7:] == ".up.sql" {
				upFiles = append(upFiles, e.Name())
			}
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := ti.Pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
	}

	return nil
}

func (ti *TestInfra) Teardown(ctx context.Context) {
	ti.Pool.Close()
	ti.Redis.Close()
	ti.pgC.Terminate(ctx)
	ti.redisC.Terminate(ctx)
}
