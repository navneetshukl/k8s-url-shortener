package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"url-shortener/internal/models"
	"url-shortener/internal/service"
)

type URLHandler struct {
	urlService service.URLService
	baseURL    string
}

func NewURLHandler(urlService service.URLService, baseURL string) *URLHandler {
	return &URLHandler{
		urlService: urlService,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (h *URLHandler) CreateShortURL(c *gin.Context) {
	var req models.CreateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	resp, err := h.urlService.CreateShortURL(c.Request.Context(), &req, h.baseURL)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "short code already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create short URL"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *URLHandler) Redirect(c *gin.Context) {
	shortCode := c.Param("shortCode")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
		return
	}

	longURL, err := h.urlService.GetLongURL(c.Request.Context(), shortCode)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve URL"})
		return
	}

	c.Redirect(http.StatusMovedPermanently, longURL)
}

func (h *URLHandler) GetStats(c *gin.Context) {
	shortCode := c.Param("shortCode")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "short code is required"})
		return
	}

	stats, err := h.urlService.GetStats(c.Request.Context(), shortCode)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *URLHandler) DeleteExpired(c *gin.Context) {
	deleted, err := h.urlService.DeleteExpired(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete expired URLs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}