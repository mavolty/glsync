-- Remove placeholder job types that were never implemented.
-- Only jira_transition is a valid job type today.
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_job_type_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_job_type_check
    CHECK (job_type IN ('jira_transition'));
