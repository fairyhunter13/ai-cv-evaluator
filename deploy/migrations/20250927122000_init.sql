-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS uploads (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN ('cv','project')),
  text TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime TEXT NOT NULL,
  size BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('queued','processing','completed','failed')),
  error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  cv_id TEXT NOT NULL REFERENCES uploads(id) ON DELETE RESTRICT,
  project_id TEXT NOT NULL REFERENCES uploads(id) ON DELETE RESTRICT,
  idempotency_key TEXT
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

CREATE TABLE IF NOT EXISTS results (
  job_id TEXT PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
  cv_match_rate DOUBLE PRECISION NOT NULL,
  cv_feedback TEXT NOT NULL,
  project_score DOUBLE PRECISION NOT NULL,
  project_feedback TEXT NOT NULL,
  overall_summary TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS results;
DROP INDEX IF EXISTS idx_jobs_status;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS uploads;
-- +goose StatementEnd
