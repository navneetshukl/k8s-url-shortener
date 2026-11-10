package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type URL struct {
	ID        int64           `json:"id"`
	ShortCode string          `json:"short_code"`
	LongURL   string          `json:"long_url"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt pgtype.Timestamptz `json:"expires_at,omitempty"`
	ClickCount int64          `json:"click_count"`
}

type CreateURLRequest struct {
	LongURL  string `json:"long_url" binding:"required,url"`
	ExpiresIn *int64 `json:"expires_in,omitempty"` // seconds from now, optional
}

type CreateURLResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

type URLStatsResponse struct {
	ShortCode  string `json:"short_code"`
	LongURL    string `json:"long_url"`
	ClickCount int64  `json:"click_count"`
	CreatedAt  string `json:"created_at"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}