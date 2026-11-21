-- IT context from the AI service (POST /cards/{id}/enrich).
-- '' means "not enriched yet", so existing cards need no backfill.
ALTER TABLE cards ADD COLUMN it_context TEXT NOT NULL DEFAULT '';
