import pytest

from errors import InvalidGradeError, parse_grade, safe_int


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("knew", "knew"),
        ("forgot", "forgot"),
        ("  KNEW \n", "knew"),
    ],
    ids=["knew", "forgot", "spaces and upper case"],
)
def test_parse_grade(text: str, expected: str) -> None:
    assert parse_grade(text) == expected


@pytest.mark.parametrize("text", ["", "maybe", "know"])
def test_parse_grade_rejects_unknown(text: str) -> None:
    with pytest.raises(InvalidGradeError, match="unknown grade"):
        parse_grade(text)


def test_invalid_grade_is_a_value_error() -> None:
    with pytest.raises(ValueError):
        parse_grade("maybe")


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("42", 42),
        ("-7", -7),
        (" 8 ", 8),
        ("abc", 0),
        ("", 0),
        ("3.5", 0),
    ],
    ids=["number", "negative", "spaces allowed", "letters", "empty", "float text"],
)
def test_safe_int(text: str, expected: int) -> None:
    assert safe_int(text, default=0) == expected
