ALTER TABLE audit_logs 
  DROP COLUMN IF EXISTS timestamp,
  DROP COLUMN IF EXISTS details_json;
