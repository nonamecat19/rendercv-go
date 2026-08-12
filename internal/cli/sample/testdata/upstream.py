"""Emit, live from the vendored Python, every starter CV the port must match.

Run by upstream_conformance_test.go against `third_party/rendercv/.venv`; prints
one JSON document on stdout and nothing else. It captures no state and writes no
file: each run asks the installed `rendercv` what it emits *now*, so a submodule
bump moves the expectation on the next test run rather than on the next rerun of
a fixture generator.

Two batteries come out of here:

1.  The finished document for each of the 9 x 22 theme/locale pairs --- the whole
    text, not a digest, so a mismatch prints the diff.
2.  The exact bytes ruamel emits for the `cv.name` region of a battery of names.
    Every row is a question about the emitter the Go port has to answer the same
    way: which style it picks (plain, single, double or literal block), how it
    escapes, and when the resolver forces a quote because the plain form would
    come back as a bool, an int, a float, a null or a timestamp.
"""

import json

from rendercv import __version__
from rendercv.schema.models.design.built_in_design import available_themes
from rendercv.schema.models.locale.locale import available_locales
from rendercv.schema.sample_generator import create_sample_yaml_input_file

# The `cv.name` battery. The eight rows of spec 013 §3.1 behavior 7 and
# upstream's own `Matías` (tests/schema/test_sample_generator.py:46) lead it.
NAMES = [
    # behavior 7's table, in its order
    "John Doe",
    "A: B",
    "*Star*",
    "#hash",
    "  pad  ",
    "",
    "yes",
    "line1\nline2",
    # upstream's unicode test, and more of the same
    "Matías",
    "Zoë  Ölçer",
    "José  ",
    "日本 語",
    "עברית",
    "emoji 😀",
    " nbsp",
    # the YAML 1.2 core resolver: quoted when the plain form is not a string
    "true",
    "True",
    "TRUE",
    "false",
    "False",
    "null",
    "Null",
    "NULL",
    "~",
    "123",
    "-5",
    "+5",
    "1.5",
    "1e3",
    "0x1F",
    "0o17",
    "0b101",
    "1_000",
    ".inf",
    "-.inf",
    ".nan",
    "<<",
    "=",
    "2020-01-01",
    "2020-01-01T00:00:00",
    # ... and plain when it is: 1.1-only bools, sexagesimals, look-alikes
    "no",
    "On",
    "OFF",
    "Y",
    "n",
    "12:30",
    "null_",
    "--",
    "a",
    # indicators: leading ones, and the two that only bite before a space
    "-",
    "- x",
    "-  x",
    "-x",
    "?",
    "? q",
    "!bang",
    "&anchor",
    "%pct",
    "@at",
    "`tick",
    "|pipe",
    ">gt",
    "{brace}",
    "[bracket]",
    "comma, sep",
    "::",
    "a: ",
    "a: b",
    "a :b",
    "a #b",
    "a# b",
    "x#y",
    "http://x.com/#f",
    "key:value",
    "--- doc",
    "...end",
    "<html>",
    # whitespace at the edges, which no plain scalar may carry
    " lead",
    "end ",
    "   ",
    # quotes: an apostrophe forces the double-quoted style
    "quote'd",
    "'quoted'",
    'dq"uote',
    '"dquoted"',
    "x\"y'z",
    "back\\slash",
    # non-printables and the characters ruamel escapes by name
    "\x07bell",
    "\x7fdel",
    "﻿bom",
    "\tstart",
    "tab\there",
    "\x1bescape",
    "\x00nul",
    # line breaks: the literal-block branch, its chomping and its indent hint
    "two\n\nlines",
    "first\n  indented",
    "  lead\nsecond",
    "tab\tstart\nsecond",
    "trailing\n",
    "end\n\n",
    "\n",
    "\n\n",
    "a\rb",
    "a\r\nb",
    " ls",
    "nel",
    "a\vb",
    "a\fb",
    # a name whose text collides with the surgery's own split markers
    # (spec 013 §5.3.3)
    "settings:\nx",
    # a nested bullet, which the §3.1 behavior 8 regex rewrites inside a name
    "x\n  - a - b",
]


def name_region(name: str) -> str:
    """The `cv.name` lines of a generated starter CV, terminated by a newline.

    `name` is the first field of `Cv`, so its region runs from just after `cv:`
    to just before `headline:` --- one line for every style but the literal
    block, which spills over several.
    """
    document = create_sample_yaml_input_file(file_path=None, name=name)
    start = document.index("cv:\n") + len("cv:\n")
    end = document.index("\n  headline:", start)
    return document[start : end + 1]


def main() -> None:
    themes = list(available_themes)
    locales = list(available_locales)

    documents = {
        f"{theme}/{locale}": create_sample_yaml_input_file(
            file_path=None, name="John Doe", theme=theme, locale=locale
        )
        for theme in themes
        for locale in locales
    }

    print(
        json.dumps(
            {
                "version": __version__,
                "themes": themes,
                "locales": locales,
                "documents": documents,
                "names": [{"name": n, "region": name_region(n)} for n in NAMES],
            },
            ensure_ascii=False,
            indent=1,
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    main()
