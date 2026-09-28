"""Inventory local release files using only the Python standard library."""

import argparse
import hashlib
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--files", type=json.loads, required=True)
    parser.add_argument("--metadata", type=json.loads, required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    if not isinstance(args.files, list) or not args.files:
        parser.error("files must be a nonempty JSON list")
    if not isinstance(args.metadata, dict) or not all(
        isinstance(k, str) and isinstance(v, str) for k, v in args.metadata.items()
    ):
        parser.error("metadata must be a JSON object with string values")

    entries = []
    seen = set()
    for name in args.files:
        if not isinstance(name, str) or not name:
            parser.error("each file must be a nonempty relative path")
        path = Path(name)
        if path.is_absolute() or ".." in path.parts or path.as_posix() != name:
            parser.error("files must use normalized relative paths without '..'")
        if name in seen:
            parser.error("duplicate file path")
        seen.add(name)
        digest = hashlib.sha256()
        size = 0
        with path.open("rb") as source:
            for chunk in iter(lambda: source.read(1024 * 1024), b""):
                digest.update(chunk)
                size += len(chunk)
        entries.append({"path": name, "size": size, "sha256": digest.hexdigest()})

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    inventory = {"format": 1, "metadata": args.metadata, "files": entries}
    output.write_text(json.dumps(inventory, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"Inventoried {len(entries)} files -> {output}")


if __name__ == "__main__":
    main()
