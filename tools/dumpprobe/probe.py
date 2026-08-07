"""Dump validated entry models as `model_dump(exclude_none=True)` gives them.

Introspects the vendored RenderCV; see tools/dumpprobe/main.go for the contract.

The cases below are chosen for the shapes the Go side cannot re-derive from a
node's text alone — an integer year against a quoted one, a `pydantic.HttpUrl`
that normalizes, an empty string that survives the dump, and an extra key that
the entry bases permit (`BaseModelWithExtraKeys`, base.py:9).
"""

import json
import sys

from rendercv.schema.models.cv.section import (
    available_entry_models,
    available_entry_type_names,
)

HEADER = (
    "GENERATED FILE — never hand-edit (AGENTS.md §10.1). Produced by "
    "tools/dumpprobe from third_party/rendercv; regenerate with `just dumpprobe`."
)

# name -> list of (case name, input mapping). One entry per model at least; the
# extra cases carry the shapes the port has to get right rather than guess.
CASES: dict[str, list[tuple[str, dict]]] = {
    "EducationEntry": [
        ("minimal", {"institution": "Boğaziçi University", "area": "Mathematics"}),
        (
            "full",
            {
                "institution": "Boğaziçi University",
                "area": "Mathematics",
                "degree": "BS",
                "start_date": "2000-09",
                "end_date": "2005-05",
                "location": "Istanbul, Türkiye",
                "summary": "A summary.",
                "highlights": ["GPA: 3.9/4.0", "Coursework: **Algebra**"],
            },
        ),
        (
            "integer_year",
            {"institution": "X", "area": "Y", "start_date": 2000, "end_date": 2005},
        ),
        ("empty_string", {"institution": "X", "area": "Y", "degree": ""}),
        ("extra_key", {"institution": "X", "area": "Y", "supervisor": "Dr. Who"}),
    ],
    "ExperienceEntry": [
        (
            "full",
            {
                "company": "Some Company",
                "position": "Software Engineer",
                "start_date": "2020-07",
                "end_date": "present",
                "location": "Remote",
                "highlights": ["Did a thing."],
            },
        ),
    ],
    "NormalEntry": [
        ("minimal", {"name": "A Normal Entry"}),
        (
            "with_date",
            {"name": "A Normal Entry", "date": "Fall 2023", "summary": "S."},
        ),
        ("year_date", {"name": "A Normal Entry", "date": 2023}),
        ("year_date_quoted", {"name": "A Normal Entry", "date": "2023"}),
    ],
    "PublicationEntry": [
        (
            "full",
            {
                "title": "A Publication",
                "authors": ["J. Doe", "R. Roe"],
                "doi": "10.1109/TASC.2023.3340648",
                "url": "https://example.com",
                "journal": "A Journal",
                "date": "2023-12",
            },
        ),
        (
            "url_only",
            {"title": "T", "authors": ["A"], "url": "https://example.com/path?a=1"},
        ),
        # A bare host is where `pydantic.HttpUrl` adds its trailing slash. The
        # `full` case cannot show it: a publication with a DOI drops its `url`
        # (publication.py's model validator), which is itself worth pinning.
        ("url_bare_host", {"title": "T", "authors": ["A"], "url": "https://example.com"}),
    ],
    "OneLineEntry": [("minimal", {"label": "Programming", "details": "Python, Go"})],
    "BulletEntry": [("minimal", {"bullet": "A bullet."})],
    "NumberedEntry": [("minimal", {"number": "A numbered item."})],
    "ReversedNumberedEntry": [
        ("minimal", {"reversed_number": "A reversed numbered item."})
    ],
}


def main():
    models = {model.__name__: model for model in available_entry_models}
    missing = set(models) - set(CASES)
    if missing:
        raise SystemExit(f"no dump case for {sorted(missing)}")

    dumps = []
    for name in available_entry_type_names:
        if name not in models:
            continue  # TextEntry is a bare `str` and has no model.
        for case, payload in CASES[name]:
            entry = models[name].model_validate(payload)
            dumps.append(
                {
                    "type": name,
                    "case": case,
                    "input": payload,
                    "dump": entry.model_dump(exclude_none=True),
                }
            )

    json.dump(
        {"_generated": HEADER, "dumps": dumps},
        sys.stdout,
        indent=2,
        ensure_ascii=False,
        default=str,
    )
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
