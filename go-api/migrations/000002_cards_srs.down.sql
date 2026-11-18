DROP INDEX IF EXISTS cards_due_at_idx;

ALTER TABLE cards
    DROP COLUMN repetitions,
    DROP COLUMN ease,
    DROP COLUMN interval_days,
    DROP COLUMN due_at;

ALTER SEQUENCE cards_id_seq RENAME TO words_id_seq;
ALTER TABLE cards RENAME CONSTRAINT cards_pkey TO words_pkey;
ALTER INDEX cards_phrase_lower_idx RENAME TO words_word_lower_idx;
ALTER TABLE cards RENAME COLUMN phrase TO word;
ALTER TABLE cards RENAME TO words;
