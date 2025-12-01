-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
  bucket_key TEXT PRIMARY KEY,
  capacity BIGINT NOT NULL,
  refill_rate DOUBLE PRECISION NOT NULL,
  tokens DOUBLE PRECISION NOT NULL,
  last_refill TIMESTAMPTZ NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS rate_limit_buckets;
-- +goose StatementEnd
