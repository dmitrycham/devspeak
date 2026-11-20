from funcs import join_words, make_counter


def test_make_counter_counts_up() -> None:
    counter = make_counter()

    assert [counter(), counter(), counter()] == [1, 2, 3]


def test_make_counter_counters_are_independent() -> None:
    first = make_counter()
    second = make_counter()

    first()
    first()

    assert second() == 1


def test_join_words_default_separator() -> None:
    assert join_words("ship", "it") == "ship it"


def test_join_words_custom_separator() -> None:
    assert join_words("go", "test", "./...", sep="_") == "go_test_./..."


def test_join_words_no_words() -> None:
    assert join_words() == ""
