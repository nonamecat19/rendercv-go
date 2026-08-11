"""Capture the starter CV's four blocks, and how ruamel emits a `cv.name`.

Run by tools/sampleprobe/main.go; prints one JSON document on stdout.

Three things come out of here, and nothing else:

1.  The four *raw* blocks `create_sample_yaml_input_file` concatenates before it
    transforms them --- `dictionary_to_yaml({"cv": ...})` and its three
    siblings (`schema/sample_generator.py:147-149`). Raw means: before the
    nested-bullet regex (`:151-159`), before the schema-hint line (`:161-166`)
    and before the comment surgery (`:168-191`). Those three are logic, and the
    Go port does them itself; this is only the data they run on.

2.  A proof that the decomposition is lossless: for every one of the 9 x 22
    theme/locale pairs, the four single-key dumps concatenated are byte-equal to
    the dump of the whole model. The probe refuses to write anything if that
    fails, because the Go generator's whole shape rests on it.

3.  The md5 of the finished document for each of the 198 pairs, plus the exact
    bytes ruamel emits for a battery of `cv.name` values. Both are fixtures the
    Go tests compare against, so the matrix and the scalar emitter are checked
    without a Python process in the loop.
"""

import hashlib
import json

from rendercv import __version__
from rendercv.schema.models.design.built_in_design import available_themes
from rendercv.schema.models.locale.locale import available_locales
from rendercv.schema.sample_generator import (
    create_sample_rendercv_pydantic_model,
    create_sample_yaml_input_file,
    dictionary_to_yaml,
    rendercv_model_to_dictionary,
)

BLOCKS = ("cv", "design", "locale", "settings")

# The `cv.name` battery. Every row is a question about ruamel's emitter that the
# Go port has to answer the same way: which style it picks (plain, single, double
# or literal block), how it escapes, and when the resolver forces a quote because
# the plain form would come back as a bool, an int, a float, a null or a
# timestamp. The eight rows of spec 013 §3.1 behavior 7 and upstream's own
# `Matías` (tests/schema/test_sample_generator.py:46) are in here by name.
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
    " nbsp",
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
    " ls",
    "nel",
    "a\vb",
    "a\fb",
    # a name whose text collides with the surgery's own split markers
    # (spec 013 §5.3.3)
    "settings:\nx",
    # a nested bullet, which the §3.1 behavior 8 regex rewrites inside a name
    "x\n  - a - b",
]


def raw_blocks(theme: str, locale: str) -> dict[str, str]:
    """The four single-key dumps, before any of the string surgery."""
    model = create_sample_rendercv_pydantic_model(
        name="John Doe", theme=theme, locale=locale
    )
    dictionary = rendercv_model_to_dictionary(model)
    return {key: dictionary_to_yaml({key: dictionary[key]}) for key in BLOCKS}


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

    design: dict[str, str] = {}
    locale_blocks: dict[str, str] = {}
    documents: dict[str, str] = {}
    cv_block: str | None = None
    settings_block: str | None = None

    for theme in themes:
        for locale in locales:
            blocks = raw_blocks(theme, locale)

            # (2) The decomposition has to be lossless, or the Go generator is
            # assembling a document upstream never emits.
            whole = dictionary_to_yaml(
                rendercv_model_to_dictionary(
                    create_sample_rendercv_pydantic_model(
                        name="John Doe", theme=theme, locale=locale
                    )
                )
            )
            joined = "".join(blocks[key] for key in BLOCKS)
            if joined != whole:
                msg = f"{theme}/{locale}: the four blocks do not rejoin the dump"
                raise SystemExit(msg)

            # The cv and settings blocks carry neither axis (behavior 14); the
            # probe asserts it rather than assuming it.
            for key, seen in (("cv", cv_block), ("settings", settings_block)):
                if seen is not None and seen != blocks[key]:
                    msg = f"{theme}/{locale}: the {key} block is not invariant"
                    raise SystemExit(msg)
            cv_block = blocks["cv"]
            settings_block = blocks["settings"]

            for key, store in (("design", design), ("locale", locale_blocks)):
                axis = theme if key == "design" else locale
                if store.get(axis, blocks[key]) != blocks[key]:
                    msg = f"{theme}/{locale}: the {key} block is not a function of {axis}"
                    raise SystemExit(msg)
                store[axis] = blocks[key]

            document = create_sample_yaml_input_file(
                file_path=None, name="John Doe", theme=theme, locale=locale
            )
            digest = hashlib.md5(document.encode("utf-8")).hexdigest()
            documents[f"{theme}/{locale}"] = digest

    print(
        json.dumps(
            {
                "version": __version__,
                "themes": themes,
                "locales": locales,
                "cv": cv_block,
                "settings": settings_block,
                "design": design,
                "locale": locale_blocks,
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
