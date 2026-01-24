#!/usr/bin/env python3
"""
PreToolUse hook: Block plan files from being saved outside feature folders.

Validates that plan-subtask-*.md files are only written to docs/features/*/ folders.
"""

import json
import os
import re
import sys


def main():
    try:
        input_data = json.load(sys.stdin)
    except json.JSONDecodeError:
        sys.exit(0)

    # Only check Write and Edit tools
    if input_data.get("tool_name") not in ("Write", "Edit"):
        sys.exit(0)

    file_path = input_data.get("tool_input", {}).get("file_path", "")

    # Check if this is a plan file
    if not re.search(r"plan-subtask-\d+\.md$", file_path):
        sys.exit(0)

    # Normalize path
    project_dir = os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd())
    if not os.path.isabs(file_path):
        file_path = os.path.abspath(file_path)

    # Get relative path from project root
    try:
        rel_path = os.path.relpath(file_path, project_dir)
    except ValueError:
        rel_path = file_path

    # Valid pattern: docs/features/{feature-folder}/plan-subtask-N.md
    valid_pattern = r"^docs/features/[^/]+/plan-subtask-\d+\.md$"

    if re.match(valid_pattern, rel_path):
        sys.exit(0)  # Valid location, allow

    # Invalid location - block with helpful message
    print(f"❌ План має бути в папці фічі!", file=sys.stderr)
    print(f"", file=sys.stderr)
    print(f"   Неправильно: {rel_path}", file=sys.stderr)
    print(f"   Правильно:   docs/features/{{feature-folder}}/plan-subtask-N.md", file=sys.stderr)
    print(f"", file=sys.stderr)
    print(f"Перевір активну фічу в CLAUDE.md і збережи план у відповідну папку.", file=sys.stderr)

    sys.exit(2)  # Block the action


if __name__ == "__main__":
    main()
