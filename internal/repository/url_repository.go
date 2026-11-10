package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"url-shortener/internal/models"
)

var (
	ErrNotFound = errors.New("url not found")
	ErrConflict = errors.New("short code already exists")
)

type URLRepository interface {
	Create(ctx context.Context, url *models.URL) error
	GetByShortCode(ctx context.Context, shortCode string) (*models.URL, error)
	GetByLongURL(ctx context.Context, longURL string) (*models.URL, error)
	IncrementClickCount(ctx context.Context, shortCode string) error
	DeleteExpired(ctx context.Context) (int64, error)
	Close()
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) URLRepository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) Create(ctx context.Context, url *models.URL) error {
	query := `
		INSERT INTO urls (short_code, long_url, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, click_count
	`
	var expiresAt interface{}
	if url.ExpiresAt.Valid {
		expiresAt = url.ExpiresAt.Time
	} else {
		expiresAt = nil
	}

	err := r.pool.QueryRow(ctx, query, url.ShortCode, url.LongURL, expiresAt).
		Scan(&url.ID, &url.CreatedAt, &url.ClickCount)
	
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r *postgresRepository) GetByShortCode(ctx context.Context, shortCode string) (*models.URL, error) {
	query := `
		SELECT id, short_code, long_url, created_at, expires_at, click_count
		FROM urls
		WHERE short_code = $1
	`
	url := &models.URL{}
	var expiresAt pgtype.Timestamptz
	
	err := r.pool.QueryRow(ctx, query, shortCode).
		Scan(&url.ID, &url.ShortCode, &url.LongURL, &url.CreatedAt, &expiresAt, &url.ClickCount)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	
	url.ExpiresAt = expiresAt
	return url, nil
}

func (r *postgresRepository) GetByLongURL(ctx context.Context, longURL string) (*models.URL, error) {
	query := `
		SELECT id, short_code, long_url, created_at, expires_at, click_count
		FROM urls
		WHERE long_url = $1
	`
	url := &models.URL{}
	var expiresAt pgtype.Timestamptz
	
	err := r.pool.QueryRow(ctx, query, longURL).
		Scan(&url.ID, &url.ShortCode, &url.LongURL, &url.CreatedAt, &expiresAt, &url.ClickCount)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	
	url.ExpiresAt = expiresAt
	return url, nil
}

func (r *postgresRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	query := `UPDATE urls SET click_count = click_count + 1 WHERE short_code = $1`
	_, err := r.pool.Exec(ctx, query, shortCode)
	return err
}

func (r *postgresRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM urls WHERE expires_at IS NOT NULL AND expires_at < NOW()`
	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *postgresRepository) Close() {
	r.pool.Close()
}

func InitDB(ctx context.Context, dsn string, maxOpenConns, maxIdleConns int, connMaxLifetime time.Duration) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	
	config.MaxConns = int32(maxOpenConns)
	config.MinConns = int32(maxIdleConns)
	config.MaxConnLifetime = connMaxLifetime
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute
	
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	
	return pool, nil
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	query := `
		CREATE TABLE IF NOT EXISTS urls (
			id BIGSERIAL PRIMARY KEY,
			short_code VARCHAR(10) UNIQUE NOT NULL,
			long_url TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			click_count BIGINT NOT NULL DEFAULT 0
		);
		
		CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);
		CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls(expires_at) WHERE expires_at IS NOT NULL;
	`
	_, err := pool.Exec(ctx, query)
	return err
}