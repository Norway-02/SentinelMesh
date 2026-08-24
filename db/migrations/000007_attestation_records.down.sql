-- Migration 000007 Down: Drop Attestation Records

ALTER TABLE agent_runs DROP COLUMN IF EXISTS attestation_id;
DROP TABLE IF EXISTS agent_attestation_records;
