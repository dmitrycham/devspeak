"""Kata 2: types. Python checks types at runtime, Go checks them at compile time."""


def describe(value: int | float | str | bool | None) -> str:
    """Return a short description of the value and its type."""
    # Like a Go type switch. bool must come before int:
    # in Python, bool is a subclass of int, so True would match `int()`.
    match value:
        case None:
            return "nothing"
        case bool():
            return f"bool: {value}"
        case int():
            return f"int: {value}"
        case float():
            return f"float: {value:.2f}"
        case str() if value == "":
            return "empty string"
        case str():
            return f"string of {len(value)} chars"
