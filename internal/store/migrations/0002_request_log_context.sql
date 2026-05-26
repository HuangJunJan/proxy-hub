ALTER TABLE request_logs ADD COLUMN endpoint TEXT;
ALTER TABLE request_logs ADD COLUMN request_type TEXT;
ALTER TABLE request_logs ADD COLUMN reasoning_effort TEXT;
ALTER TABLE request_logs ADD COLUMN billing_mode TEXT;
ALTER TABLE request_logs ADD COLUMN first_token_ms INTEGER;
ALTER TABLE request_logs ADD COLUMN user_agent TEXT;
