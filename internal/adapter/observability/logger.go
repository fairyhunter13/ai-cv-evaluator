package observability

import (
	"log/slog"
	"os"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// SetupLogger configures a JSON slog logger with environment fields.
func SetupLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{}
	// In dev, show debug level; in prod, default to info
	if cfg.IsDev() {
		opts.Level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(h).With(
		slog.String("service", cfg.OTELServiceName),
		slog.String("env", cfg.AppEnv),
	)
	return logger
}
