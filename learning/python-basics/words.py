"""Kata 1: count words in a text."""

PUNCTUATION = ".,!?"


def count_words(text: str) -> dict[str, int]:
    """Count how many times each word appears in the text.

    Words are case-insensitive. Punctuation at the start and end
    of a word is removed.
    """
    counts: dict[str, int] = {}
    for raw_word in text.split():
        word = raw_word.lower().strip(PUNCTUATION)
        if not word:
            # The "word" was only punctuation, for example "!!!".
            continue
        counts[word] = counts.get(word, 0) + 1
    return counts


def top_words(text: str, n: int) -> list[tuple[str, int]]:
    """Return the n most frequent words.

    A higher count comes first. Equal counts are sorted alphabetically.
    """
    if n <= 0:
        return []
    counts = count_words(text)
    # Sort key: -count puts big counts first, word breaks ties A→Z.
    ranked = sorted(counts.items(), key=lambda item: (-item[1], item[0]))
    return ranked[:n]
