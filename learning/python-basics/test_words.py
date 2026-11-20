import pytest

from words import count_words, top_words


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("Go is fast. Go is simple!", {"go": 2, "is": 2, "fast": 1, "simple": 1}),
        ("", {}),
        ("   \n\t  ", {}),
        ("Python PYTHON python", {"python": 3}),
        ("hello, world! hello?", {"hello": 2, "world": 1}),
        ("!!! ? ...", {}),
        ("line one\nline two", {"line": 2, "one": 1, "two": 1}),
    ],
    ids=[
        "example",
        "empty",
        "whitespace only",
        "mixed case",
        "punctuation",
        "only punctuation",
        "newlines",
    ],
)
def test_count_words(text: str, expected: dict[str, int]) -> None:
    assert count_words(text) == expected


@pytest.mark.parametrize(
    ("text", "n", "expected"),
    [
        ("b a b c a b", 2, [("b", 3), ("a", 2)]),
        ("c b a", 3, [("a", 1), ("b", 1), ("c", 1)]),
        ("go go rust", 10, [("go", 2), ("rust", 1)]),
        ("go go rust", 0, []),
        ("go go rust", -1, []),
        ("", 3, []),
    ],
    ids=[
        "example",
        "ties sorted alphabetically",
        "n bigger than word count",
        "n is zero",
        "n is negative",
        "empty text",
    ],
)
def test_top_words(text: str, n: int, expected: list[tuple[str, int]]) -> None:
    assert top_words(text, n) == expected
