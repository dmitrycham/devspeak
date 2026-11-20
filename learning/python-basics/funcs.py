"""Kata 4: functions."""

from collections.abc import Callable


def make_counter() -> Callable[[], int]:
    """Return a function that returns 1, 2, 3, ... on each call."""
    count = 0

    def next_value() -> int:
        # Without nonlocal, `count += 1` would create a new local variable.
        # Go closures capture variables by reference, so Go needs no keyword.
        nonlocal count
        count += 1
        return count

    return next_value


def join_words(*words: str, sep: str = " ") -> str:
    """Join any number of words. sep is keyword-only because it comes after *words."""
    # *words is like Go's `words ...string`; inside, it is a tuple.
    return sep.join(words)
