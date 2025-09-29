//go:build never

// Package httpserver provides HTTP server adapters. This file contains a stub
// admin server used when building without the 'adminui' build tag.
package httpserver

import (
	"github.com/go-chi/chi/v5"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// AdminServer is a no-op stub for builds without the adminui tag.
// This avoids pulling heavy template parsing into default test builds.
type AdminServer struct{}

// NewAdminServer returns a no-op admin server when built without the adminui tag.
func NewAdminServer(_ config.Config, _ *Server) (*AdminServer, error) { return &AdminServer{}, nil }

// MountRoutes mounts no routes in the stub implementation.
func (a *AdminServer) MountRoutes(_ chi.Router) {}
