-- Runs once per cluster (cmd/migrate records it in schema_once).
--
-- Until this release every refused /v1/decisions call also raised the
-- account's free_decisions counter, so accounts refused a few times sat far
-- above FREE_DECISIONS and never got a free call again, whatever the limit.
-- Approved by Renato (30/09/2026): zero the counter of every account. With no
-- row, the next call starts again at 1. decision_usage (daily) is untouched.

DELETE FROM free_decisions;
