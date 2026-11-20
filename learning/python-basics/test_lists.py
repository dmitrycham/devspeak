import pytest

from lists import group_by_first_letter, unique_in_order


@pytest.mark.parametrize(
    ("items", "expected"),
    [
        (["go", "rust", "go", "python", "rust"], ["go", "rust", "python"]),
        ([], []),
        (["a", "a", "a"], ["a"]),
        (["Go", "go"], ["Go", "go"]),
    ],
    ids=["example", "empty", "all the same", "case-sensitive"],
)
def test_unique_in_order(items: list[str], expected: list[str]) -> None:
    assert unique_in_order(items) == expected


def test_unique_in_order_does_not_change_input() -> None:
    items = ["b", "a", "b"]

    unique_in_order(items)

    assert items == ["b", "a", "b"]


@pytest.mark.parametrize(
    ("words", "expected"),
    [
        (
            ["go", "Gopher", "rust", "ruby"],
            {"g": ["go", "Gopher"], "r": ["rust", "ruby"]},
        ),
        ([], {}),
        (["", "api"], {"a": ["api"]}),
    ],
    ids=["example", "empty", "empty word skipped"],
)
def test_group_by_first_letter(
    words: list[str], expected: dict[str, list[str]]
) -> None:
    assert group_by_first_letter(words) == expected
