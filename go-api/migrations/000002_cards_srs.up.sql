-- Rename words → cards and word → phrase.
-- The unique index on lower(word) follows the column: it becomes lower(phrase).
ALTER TABLE words RENAME TO cards;
ALTER TABLE cards RENAME COLUMN word TO phrase;
ALTER INDEX words_word_lower_idx RENAME TO cards_phrase_lower_idx;
ALTER TABLE cards RENAME CONSTRAINT words_pkey TO cards_pkey;
ALTER SEQUENCE words_id_seq RENAME TO cards_id_seq;

-- Spaced repetition (SM-2) state. Existing and new cards are due now.
ALTER TABLE cards
    ADD COLUMN due_at        TIMESTAMPTZ      NOT NULL DEFAULT now(),
    ADD COLUMN interval_days INTEGER          NOT NULL DEFAULT 0,
    ADD COLUMN ease          DOUBLE PRECISION NOT NULL DEFAULT 2.5,
    ADD COLUMN repetitions   INTEGER          NOT NULL DEFAULT 0;

-- Speeds up "next due card": WHERE due_at <= now ORDER BY due_at.
CREATE INDEX cards_due_at_idx ON cards (due_at);
