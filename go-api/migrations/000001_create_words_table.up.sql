CREATE TABLE words (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    word        TEXT NOT NULL,
    translation TEXT NOT NULL,
    example     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness: "Go" and "go" are the same word.
CREATE UNIQUE INDEX words_word_lower_idx ON words (lower(word));
