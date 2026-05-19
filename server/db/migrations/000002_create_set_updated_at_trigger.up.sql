CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Helper used by ck_user_settings_lead_days_sane.
-- Postgres CHECK constraints cannot contain subqueries, so we use an immutable
-- function that wraps the unnest check.
CREATE OR REPLACE FUNCTION int_array_all_in_range(arr int[], lo int, hi int)
RETURNS boolean AS $$
SELECT NOT EXISTS (SELECT 1 FROM unnest(arr) AS v WHERE v < lo OR v > hi);
$$ LANGUAGE sql IMMUTABLE;
