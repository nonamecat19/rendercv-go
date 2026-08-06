"""Dump ruamel lc.data coordinates for a YAML file as JSON."""
import json
import pathlib
import sys

from ruamel.yaml.comments import CommentedMap, CommentedSeq

from rendercv.schema.yaml_reader import read_yaml


def dump_coords(obj, prefix=""):
    """Walk a CommentedMap/CommentedSeq and collect lc.data entries."""
    result = {}

    if isinstance(obj, CommentedMap):
        for key, value in obj.items():
            path = f"{prefix}.{key}" if prefix else key
            if key in obj.lc.data:
                coords = obj.lc.data[key]
                result[path] = list(coords)
            if isinstance(value, (CommentedMap, CommentedSeq)):
                result.update(dump_coords(value, path))
            elif isinstance(value, list):
                for i, item in enumerate(value):
                    item_path = f"{path}.{i}"
                    if isinstance(item, (CommentedMap, CommentedSeq)):
                        result.update(dump_coords(item, item_path))
    elif isinstance(obj, CommentedSeq):
        for i, item in enumerate(obj):
            item_path = f"{prefix}.{i}"
            if i in obj.lc.data:
                coords = obj.lc.data[i]
                result[item_path] = list(coords)
            if isinstance(item, (CommentedMap, CommentedSeq)):
                result.update(dump_coords(item, item_path))

    return result


def main():
    path = pathlib.Path(sys.argv[1])
    data = read_yaml(path)
    coords = dump_coords(data)
    json.dump(coords, sys.stdout, indent=2, sort_keys=True)


if __name__ == "__main__":
    main()
