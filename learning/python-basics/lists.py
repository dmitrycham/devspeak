"""Kata 3: lists and dicts."""


def unique_in_order(items: list[str]) -> list[str]:
    """Remove duplicates and keep the first occurrence of each item."""
    # A set is like Go's map[string]struct{}: fast membership check, no order.
    seen: set[str] = set()
    result: list[str] = []
    for item in items:
        if item not in seen:
            seen.add(item)
            result.append(item)
    return result


def group_by_first_letter(words: list[str]) -> dict[str, list[str]]:
    """Group words by their first letter, lower case. Empty words are skipped."""
    groups: dict[str, list[str]] = {}
    for word in words:
        if not word:
            continue
        # setdefault creates the empty list on first use,
        # like `groups[k] = append(groups[k], w)` on a nil slice in Go.
        groups.setdefault(word[0].lower(), []).append(word)
    return groups
