ALTER TABLE run_scheduling_assignments DROP COLUMN IF EXISTS execution_generation;
ALTER TABLE run_scheduling_assignments DROP COLUMN IF EXISTS fencing_token;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS execution_generation;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS fencing_token;
DROP TABLE IF EXISTS clusters;
