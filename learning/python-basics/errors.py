"""Kata 6: exceptions. Python raises errors, Go returns them as values."""

from typing import Literal

Grade = Literal["knew", "forgot"]


# Subclassing ValueError lets callers catch it as a general "bad value" too,
# like errors.Is with a wrapped sentinel error in Go.
class InvalidGradeError(ValueError):
    pass


def parse_grade(text: str) -> Grade:
    """Parse a review grade, like ParseGrade in go-api/internal/srs."""
    match text.strip().lower():
        case "knew":
            return "knew"
        case "forgot":
            return "forgot"
        case _:
            raise InvalidGradeError(f"unknown grade: {text!r}")


def safe_int(text: str, default: int) -> int:
    """Convert text to int, or return default if it is not a number."""
    # Go: n, err := strconv.Atoi(text); if err != nil { return def }
    try:
        return int(text)
    except ValueError:
        return default
