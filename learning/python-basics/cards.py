"""Kata 5: classes. A dataclass is the closest thing to a Go struct."""

from dataclasses import dataclass
from datetime import datetime


# frozen=True makes fields read-only: a Card acts like a Go struct passed by value.
@dataclass(frozen=True)
class Card:
    phrase: str
    translation: str
    due_at: datetime

    # `self` is the receiver, like `func (c Card) IsDue(now time.Time) bool`.
    def is_due(self, now: datetime) -> bool:
        """A card is due when its review time has come."""
        return self.due_at <= now
