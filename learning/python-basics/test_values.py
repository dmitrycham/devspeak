import pytest

from values import describe


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        (None, "nothing"),
        (True, "bool: True"),
        (False, "bool: False"),
        (0, "int: 0"),
        (42, "int: 42"),
        (3.14159, "float: 3.14"),
        ("", "empty string"),
        ("Go", "string of 2 chars"),
    ],
    ids=[
        "none",
        "true is bool, not int",
        "false is bool, not int",
        "zero",
        "int",
        "float rounded",
        "empty string",
        "string",
    ],
)
def test_describe(value: int | float | str | bool | None, expected: str) -> None:
    assert describe(value) == expected
