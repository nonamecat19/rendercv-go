"""Dump how upstream formats a phone number in each of the three formats.

Introspects the vendored RenderCV's own dependencies; see
tools/phoneprobe/main.go for the contract.

Two steps, both of which the port has to reproduce: `PhoneNumber` validates and
stores the RFC 3966 form, and `parse_connections` then formats **that stored
string** through `phonenumbers.format_number` (connections.py:96-110).
"""

import json
import sys

import phonenumbers
import pydantic
from pydantic_extra_types import phone_numbers

HEADER = (
    "GENERATED FILE — never hand-edit (AGENTS.md §10.1). Produced by "
    "tools/phoneprobe from third_party/rendercv; regenerate with `just phoneprobe`."
)

# The two the corpus carries, the one `render_override_scalar` passes on the
# command line, and four more regions whose national grouping differs from the
# international one — which is the whole point of the option.
NUMBERS = [
    "+1-415-555-0142",
    "+34-612-345-678",
    "+1-555-555-5555",
    "+90 541 999 99 99",
    "+44 20 7946 0958",
    "+81 3-1234-5678",
    "+49 30 901820",
]

FORMATS = ["national", "international", "E164"]


class Holder(pydantic.BaseModel):
    phone: phone_numbers.PhoneNumber


def main():
    rows = []
    for number in NUMBERS:
        try:
            stored = str(Holder(phone=number).phone)
        except pydantic.ValidationError:
            rows.append({"input": number, "valid": False})
            continue

        formatted = {}
        for name in FORMATS:
            formatted[name] = phonenumbers.format_number(
                phonenumbers.parse(stored, None),
                getattr(phonenumbers.PhoneNumberFormat, name.upper()),
            )
        rows.append(
            {"input": number, "valid": True, "stored": stored, "formatted": formatted}
        )

    json.dump(
        {"_generated": HEADER, "numbers": rows},
        sys.stdout,
        indent=2,
        ensure_ascii=False,
    )
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
