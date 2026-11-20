import dataclasses
from datetime import UTC, datetime, timedelta

import pytest

from cards import Card

NOW = datetime(2026, 11, 20, 12, 0, tzinfo=UTC)


def make_card(due_at: datetime) -> Card:
    return Card(phrase="ship it", translation="выкатывай", due_at=due_at)


@pytest.mark.parametrize(
    ("due_at", "expected"),
    [
        (NOW - timedelta(days=1), True),
        (NOW, True),
        (NOW + timedelta(minutes=1), False),
    ],
    ids=["in the past", "exactly now", "in the future"],
)
def test_card_is_due(due_at: datetime, expected: bool) -> None:
    assert make_card(due_at).is_due(NOW) is expected


def test_cards_with_same_fields_are_equal() -> None:
    # dataclass generates __eq__, like == on comparable Go structs.
    assert make_card(NOW) == make_card(NOW)


def test_card_is_frozen() -> None:
    card = make_card(NOW)

    with pytest.raises(dataclasses.FrozenInstanceError):
        card.phrase = "changed"  # type: ignore[misc]
