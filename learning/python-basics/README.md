# Python basics katas

Small Python functions with tests, each compared with Go (DEV-9).

| File | Topic | Functions |
|---|---|---|
| `words.py` | list/dict | `count_words`, `top_words` |
| `values.py` | types | `describe` |
| `lists.py` | list/dict | `unique_in_order`, `group_by_first_letter` |
| `funcs.py` | functions | `make_counter`, `join_words` |
| `cards.py` | classes | `Card.is_due` |
| `errors.py` | exceptions | `parse_grade`, `safe_int` |

## Run

```sh
cd learning/python-basics
uv run pytest
uv run ruff check .
```
