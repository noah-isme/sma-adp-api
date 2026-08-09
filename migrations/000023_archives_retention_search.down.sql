-- Migration Down: 000023_archives_retention_search.down.sql

DROP TABLE IF EXISTS archive_audit_log;
DROP TABLE IF EXISTS archive_documents;
DROP TABLE IF EXISTS retention_policies;
