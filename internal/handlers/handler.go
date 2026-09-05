package handlers

import (
	"Gocorecms/internal/config"
)

// Handler bundles dependencies for all HTTP handlers.
type Handler struct {
	Cfg *config.Config
}

// New builds the handler set.
func New(cfg *config.Config) *Handler {
	return &Handler{Cfg: cfg}
}
