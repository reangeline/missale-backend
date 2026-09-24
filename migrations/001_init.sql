-- Aurora DSQL: one DDL statement per transaction, no foreign keys or sequences.
-- Run as admin with `make migrate` (cmd/migrate), which fills in {{LAMBDA_ROLE_ARN}}.

CREATE TABLE IF NOT EXISTS users (
    id         text        PRIMARY KEY,   -- Cognito sub
    apple_sub  text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS decision_usage (
    user_id text NOT NULL,
    day     date NOT NULL,
    calls   int  NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

CREATE ROLE missale_api WITH LOGIN;

AWS IAM GRANT missale_api TO '{{LAMBDA_ROLE_ARN}}';

GRANT SELECT, INSERT, UPDATE, DELETE ON users, decision_usage TO missale_api;
