DROP TABLE IF EXISTS agent_checkpoints;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS recovery_generation;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS recovered_from_checkpoint_id;
