#!/usr/bin/env python3
"""Validate pull request titles and required description sections."""

from __future__ import annotations

import json
import os
import re
import subprocess
from pathlib import Path


SECTIONS = ("Summary", "Validation", "Operational impact")
COMMENT_PATTERN = re.compile(r"<!--.*?-->", re.DOTALL)


def extract_section(body: str, name: str) -> str | None:
    heading = re.compile(
        rf"^##[ \t]+{re.escape(name)}[ \t]*\r?\n(.*?)(?=^##[ \t]+|\Z)",
        re.MULTILINE | re.DOTALL,
    )
    match = heading.search(body)
    return match.group(1) if match else None


def section_has_content(section: str, *, allow_checked_boxes: bool = False) -> bool:
    for line in section.splitlines():
        line = line.strip()
        if not line:
            continue
        if re.match(r"[-*+] \[[ xX]\](?:\s|$)", line):
            if allow_checked_boxes and re.match(r"[-*+] \[[xX]\]", line):
                return True
            continue
        return True
    return False


def main() -> int:
    event_path = Path(os.environ["GITHUB_EVENT_PATH"])
    event = json.loads(event_path.read_text(encoding="utf-8"))
    pull_request = event.get("pull_request") or {}
    title = pull_request.get("title") or ""
    body = pull_request.get("body") or ""

    validator = Path(__file__).resolve().with_name("validate-commit-subject.sh")
    result = subprocess.run(["bash", str(validator), title], check=False)
    if result.returncode:
        return result.returncode

    # Release Please generates its own body and owns the release checklist; keep
    # validating its Conventional Commit title, but don't require our author template.
    head = pull_request.get("head") or {}
    if (head.get("ref") or "").startswith("release-please--branches--"):
        return 0

    body = COMMENT_PATTERN.sub("", body)
    failed = False
    for name in SECTIONS:
        section = extract_section(body, name)
        if section is None:
            print(f"PR description must include a '## {name}' section.")
            failed = True
            continue
        if not section_has_content(section, allow_checked_boxes=name == "Validation"):
            print(f"Add a short description under '## {name}'.")
            failed = True

    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
