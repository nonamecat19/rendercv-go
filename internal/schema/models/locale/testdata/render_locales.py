"""Render one CV per locale, live from the vendored Python.

Run by locale_matrix_conformance_test.go against `third_party/rendercv/.venv`;
prints one JSON document on stdout and nothing else. It captures no state and
writes no file the caller keeps: each run asks the installed `rendercv` what it
renders *now*, so a submodule bump moves the expectation on the next test run
rather than on the next rerun of a fixture generator.

Argument: the path to `locale_matrix.yaml`. The `locale:` block is prepended
here, one language at a time, so the fixture stays a plain CV.

`date` comes back with the artifacts because the footer carries
`locale.last_updated` next to today's date. A run that crosses midnight would
otherwise look like a parity failure in the footer, and the Go side skips on
exactly that.
"""

import datetime
import json
import pathlib
import sys
import tempfile

from rendercv.renderer.html import generate_html
from rendercv.renderer.markdown import generate_markdown
from rendercv.renderer.typst import generate_typst
from rendercv.schema.models.locale.locale import available_locales
from rendercv.schema.rendercv_model_builder import (
    build_rendercv_dictionary_and_model,
)


def render(cv_yaml: str, language: str, workdir: pathlib.Path) -> dict[str, str]:
    input_file = workdir / "CV.yaml"
    input_file.write_text(
        cv_yaml.replace("cv:\n", f"locale:\n  language: {language}\ncv:\n", 1),
        encoding="utf-8",
    )
    _, model = build_rendercv_dictionary_and_model(
        input_file.read_text(encoding="utf-8"),
        input_file_path=input_file,
        dont_generate_pdf=True,
        dont_generate_png=True,
    )
    typst_path = generate_typst(model)
    markdown_path = generate_markdown(model)
    html_path = generate_html(model, markdown_path)
    return {
        "typst": typst_path.read_text(encoding="utf-8"),
        "markdown": markdown_path.read_text(encoding="utf-8"),
        "html": html_path.read_text(encoding="utf-8"),
    }


def main() -> None:
    cv_yaml = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
    documents: dict[str, dict[str, str]] = {}
    with tempfile.TemporaryDirectory() as raw:
        workdir = pathlib.Path(raw)
        for language in available_locales:
            documents[language] = render(cv_yaml, language, workdir)

    json.dump(
        {
            "date": datetime.date.today().isoformat(),
            "locales": list(available_locales),
            "documents": documents,
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
