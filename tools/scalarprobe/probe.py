"""Dump the Python type ruamel resolves each scalar to, per scalar style.

The corpus is written here rather than in Go so the expectations come from
upstream's own reader (spec 004, STATE.md cut-scope item 5).
"""
import json
import pathlib
import sys
import tempfile

from rendercv.schema.yaml_reader import read_yaml

RAW = [
    "", "~", "null", "Null", "NULL",
    "true", "True", "TRUE", "false", "False", "FALSE",
    ".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF",
    "-.inf", "-.Inf", "-.INF", ".nan", ".NaN", ".NAN",
    "0", "42", "-7", "+7", "0x1F", "0o17", "1_000", "00123",
    "3.14", "-3.14", "1e10", "1E10", "1.0e-3", ".5", "5.",
    "2020", "2020-09", "2020-09-24", "2020-09-24T10:00:00Z",
    "hello", "yes", "no", "on", "off", "y", "n",
    "None", "NONE", "nan", "inf", "Infinity",
    "0b101", "0777", "1:30", "a b c",
    # "-", "?" and "!" are YAML indicators, not scalars: bare, they are a
    # scanner error or a tag, and the reader never routes them to ResolveScalar.
]

STYLES = {
    "plain": "{}",
    "single": "'{}'",
    "double": '"{}"',
}

def kind(value):
    """Map a resolved value to a node kind.

    isinstance, not type(): ruamel's round-trip loader returns int and float
    subclasses (ScalarInt, ScalarFloat, HexCapsInt, OctalInt, BinaryInt) that
    carry the original formatting. bool is checked first because it is a
    subclass of int.
    """
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, int):
        return "int"
    if isinstance(value, float):
        return "float"
    if isinstance(value, str):
        return "string"
    return type(value).__name__


def main():
    rows = []
    directory = pathlib.Path(tempfile.mkdtemp())
    for raw in RAW:
        for style, template in STYLES.items():
            if style != "plain" and ("'" in raw or '"' in raw):
                continue
            path = directory / "probe.yaml"
            path.write_text("k: " + template.format(raw) + "\n")
            try:
                value = read_yaml(path)["k"]
            except Exception as error:  # noqa: BLE001 - recorded, not raised
                rows.append({"raw": raw, "style": style, "kind": "error",
                             "detail": type(error).__name__})
                continue
            rows.append({"raw": raw, "style": style, "kind": kind(value)})

    json.dump(rows, sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
