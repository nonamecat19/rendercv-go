"""Dump the entry models' field orders and discrimination table as JSON.

Introspects the vendored RenderCV; see tools/entryprobe/main.go for the contract.
Ordering is the point of this fixture: field lists keep pydantic's declaration
order and the type list keeps the `EntryModel` union order, so nothing here is
sorted except the members of each characteristic-field set.
"""

import json
import sys

from rendercv.schema.models.cv.section import (
    available_entry_models,
    available_entry_type_names,
    characteristic_entry_fields,
)

HEADER = (
    "GENERATED FILE — never hand-edit (AGENTS.md §10.1). Produced by "
    "tools/entryprobe from third_party/rendercv; regenerate with `just entryprobe`."
)


def main():
    payload = {
        "_generated": HEADER,
        "available_entry_type_names": list(available_entry_type_names),
        "field_orders": [
            {"type": model.__name__, "fields": list(model.model_fields.keys())}
            for model in available_entry_models
        ],
        "characteristic_entry_fields": [
            {
                "type": model.__name__,
                "fields": sorted(characteristic_entry_fields[model]),
            }
            for model in available_entry_models
        ],
    }
    json.dump(payload, sys.stdout, indent=2, ensure_ascii=False)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
