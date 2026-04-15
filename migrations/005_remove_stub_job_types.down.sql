ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_job_type_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_job_type_check
    CHECK (job_type IN ('jira_transition', 'larkbase_sync', 'feishu_notify'));
