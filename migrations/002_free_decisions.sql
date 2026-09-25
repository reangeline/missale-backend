-- The orientação the onboarding offers without a subscription: lifetime Jev
-- calls per account, checked against FREE_DECISIONS.

CREATE TABLE IF NOT EXISTS free_decisions (
    user_id text PRIMARY KEY,
    calls   int  NOT NULL DEFAULT 0
);

GRANT SELECT, INSERT, UPDATE, DELETE ON free_decisions TO missale_api;
