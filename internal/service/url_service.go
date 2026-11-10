package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"url-shortener/internal/models"
	"url-shortener/internal/repository"
)

var (
	ErrNotFound = repository.ErrNotFound
	ErrConflict = repository.ErrConflict
)

type URLService interface {
	CreateShortURL(ctx context.Context, req *models.CreateURLRequest, baseURL string) (*models.CreateURLResponse, error)
	GetLongURL(ctx context.Context, shortCode string) (string, error)
	GetStats(ctx context.Context, shortCode string) (*models.URLStatsResponse, error)
	DeleteExpired(ctx context.Context) (int64, error)
}

type urlService struct {
	urlRepo  repository.URLRepository
	redisRepo repository.RedisRepository
	cacheTTL time.Duration
}

func NewURLService(urlRepo repository.URLRepository, redisRepo repository.RedisRepository, cacheTTL time.Duration) URLService {
	return &urlService{
		urlRepo:   urlRepo,
		redisRepo: redisRepo,
		cacheTTL:  cacheTTL,
	}
}

func (s *urlService) CreateShortURL(ctx context.Context, req *models.CreateURLRequest, baseURL string) (*models.CreateURLResponse, error) {
	// Check if URL already exists
	existing, err := s.urlRepo.GetByLongURL(ctx, req.LongURL)
	if err == nil {
		return &models.CreateURLResponse{
			ShortCode: existing.ShortCode,
			ShortURL:  baseURL + "/" + existing.ShortCode,
		}, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	// Generate unique short code
	shortCode, err := s.generateUniqueShortCode(ctx)
	if err != nil {
		return nil, err
	}

	// Create URL model
	url := &models.URL{
		ShortCode: shortCode,
		LongURL:   req.LongURL,
	}
	
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		url.ExpiresAt = pgtype.Timestamptz{
			Time:  time.Now().Add(time.Duration(*req.ExpiresIn) * time.Second),
			Valid: true,
		}
	}

	// Save to database
	if err := s.urlRepo.Create(ctx, url); err != nil {
		return nil, err
	}

	// Cache in Redis
	if err := s.redisRepo.SetURL(ctx, url, s.cacheTTL); err != nil {
		// Log error but don't fail the request
		// In production, use a proper logger
	}

	return &models.CreateURLResponse{
		ShortCode: shortCode,
		ShortURL:  baseURL + "/" + shortCode,
	}, nil
}

func (s *urlService) GetLongURL(ctx context.Context, shortCode string) (string, error) {
	// Try Redis first
	url, err := s.redisRepo.GetURL(ctx, shortCode)
	if err == nil {
		// Increment click count asynchronously
		go func() {
			_ = s.urlRepo.IncrementClickCount(context.Background(), shortCode)
			_, _ = s.redisRepo.IncrementClickCount(context.Background(), shortCode)
		}()
		return url.LongURL, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}

	// Fallback to database
	url, err = s.urlRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return "", err
	}

	// Check expiration
	if url.ExpiresAt.Valid && url.ExpiresAt.Time.Before(time.Now()) {
		return "", ErrNotFound
	}

	// Cache in Redis
	_ = s.redisRepo.SetURL(ctx, url, s.cacheTTL)

	// Increment click count asynchronously
	go func() {
		_ = s.urlRepo.IncrementClickCount(context.Background(), shortCode)
		_, _ = s.redisRepo.IncrementClickCount(context.Background(), shortCode)
	}()

	return url.LongURL, nil
}

func (s *urlService) GetStats(ctx context.Context, shortCode string) (*models.URLStatsResponse, error) {
	url, err := s.urlRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	// Try to get click count from Redis
	if clickCount, err := s.redisRepo.IncrementClickCount(ctx, shortCode); err == nil {
		url.ClickCount = clickCount
	}

	resp := &models.URLStatsResponse{
		ShortCode:  url.ShortCode,
		LongURL:    url.LongURL,
		ClickCount: url.ClickCount,
		CreatedAt:  url.CreatedAt.Format(time.RFC3339),
	}
	
	if url.ExpiresAt.Valid {
		resp.ExpiresAt = url.ExpiresAt.Time.Format(time.RFC3339)
	}

	return resp, nil
}

func (s *urlService) DeleteExpired(ctx context.Context) (int64, error) {
	deleted, err := s.urlRepo.DeleteExpired(ctx)
	if err != nil {
		return 0, err
	}
	// Note: In a production system, you'd also want to clean up Redis cache
	// This would require scanning Redis keys which is not efficient
	// Consider using Redis TTL for automatic expiration
	return deleted, nil
}

func (s *urlService) generateUniqueShortCode(ctx context.Context) (string, error) {
	const maxRetries = 10
	for i := 0; i < maxRetries; i++ {
		code, err := generateShortCode(7)
		if err != nil {
			return "", err
		}
		
		_, err = s.urlRepo.GetByShortCode(ctx, code)
		if errors.Is(err, repository.ErrNotFound) {
			return code, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("failed to generate unique short code after max retries")
}

func generateShortCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use URL-safe base64 encoding
	encoded := base64.URLEncoding.EncodeToString(bytes)
	// Remove padding and truncate to desired length
	if len(encoded) > length {
		encoded = encoded[:length]
	}
	return encoded, nil
}