#!/usr/bin/env python3
"""Validate JSON schemas in schemas/ and check example files in examples/."""

from __future__ import annotations

import glob
import json
import os
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("Error: pyyaml is required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)

try:
    from jsonschema import Draft7Validator, ValidationError
except ImportError:
    print("Error: jsonschema is required. Install with: pip install jsonschema", file=sys.stderr)
    sys.exit(1)


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent
    schemas_dir = repo_root / "schemas"
    examples_dir = repo_root / "examples"

    schema_files = sorted(glob.glob(str(schemas_dir / "*.json")))
    if not schema_files:
        print(f"No schemas found in {schemas_dir}", file=sys.stderr)
        return 1

    schemas: dict[str, dict] = {}
    failed = False

    # 1. Validate JSON schema syntax and Draft-7 compliance
    for s_path in schema_files:
        p = Path(s_path)
        try:
            with open(p, "r", encoding="utf-8") as f:
                schema = json.load(f)
            Draft7Validator.check_schema(schema)
            schemas[p.stem] = schema
            print(f"✓ Valid schema: {p.relative_to(repo_root)}")
        except Exception as e:
            print(f"✗ Invalid schema {p.relative_to(repo_root)}: {e}", file=sys.stderr)
            failed = True

    if failed:
        return 1

    # 2. Validate example YAML files against matching schemas
    example_mappings = {
        "cluster.yaml": "cluster",
        "service.yaml": "service",
        "express-service.yaml": "express-service",
        "gateway.yaml": "gateway",
    }

    for ex_name, schema_key in example_mappings.items():
        ex_path = examples_dir / ex_name
        if not ex_path.exists():
            continue

        if schema_key not in schemas:
            print(f"Warning: No schema {schema_key}.json found for example {ex_name}", file=sys.stderr)
            continue

        schema = schemas[schema_key]
        validator = Draft7Validator(schema)

        try:
            with open(ex_path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f)
            validator.validate(data)
            print(f"✓ Valid example: {ex_path.relative_to(repo_root)} (against {schema_key}.json)")
        except ValidationError as e:
            print(f"✗ Example validation failed for {ex_path.relative_to(repo_root)}: {e.message}", file=sys.stderr)
            failed = True
        except Exception as e:
            print(f"✗ Error reading example {ex_path.relative_to(repo_root)}: {e}", file=sys.stderr)
            failed = True

    if failed:
        return 1

    print("All schemas and examples passed validation.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
