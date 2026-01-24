#!/usr/bin/env python3
"""
SessionStart hook: Display current feature status at session start.

Reads CLAUDE.md to find active feature, then shows:
- Feature name and status
- Current subtask (first unchecked)
- Whether plan exists for current subtask
"""

import json
import os
import re
import sys
from typing import Optional


def find_active_feature(claude_md_content: str) -> Optional[str]:
    """Extract active feature path from CLAUDE.md."""
    match = re.search(r"\*\*Active feature:\*\*.*?\[.*?\]\((docs/features/[^)]+)\)", claude_md_content)
    if match:
        return match.group(1)
    return None


def parse_feature_readme(readme_content: str) -> dict:
    """Parse feature README.md to extract status info."""
    result = {
        "name": None,
        "status": None,
        "current_subtask": None,
        "current_subtask_num": None,
        "has_plan": False,
        "plan_file": None,
        "plan_file_exists": False,
    }

    # Extract feature name from first heading
    name_match = re.search(r"^# (.+)$", readme_content, re.MULTILINE)
    if name_match:
        result["name"] = name_match.group(1)

    # Extract status
    status_match = re.search(r"\*\*Status:\*\*\s*(.+)$", readme_content, re.MULTILINE)
    if status_match:
        result["status"] = status_match.group(1).strip()

    # Find subtasks section and locate first unchecked item
    subtask_pattern = r"\*\*Subtask (\d+):\*\*\s*([^\n]+)"
    checkbox_pattern = r"- \[ \]"

    subtasks = list(re.finditer(subtask_pattern, readme_content))

    for i, subtask_match in enumerate(subtasks):
        subtask_num = subtask_match.group(1)
        subtask_title = subtask_match.group(2).strip()

        # Find the section between this subtask and the next (or end)
        start_pos = subtask_match.end()
        end_pos = subtasks[i + 1].start() if i + 1 < len(subtasks) else len(readme_content)
        section = readme_content[start_pos:end_pos]

        # Check if this subtask has unchecked items
        if re.search(checkbox_pattern, section):
            result["current_subtask"] = subtask_title
            result["current_subtask_num"] = subtask_num

            # Check if plan exists (look for plan link in subtask title line)
            plan_match = re.search(r"\[plan-subtask-\d+\.md\]\((plan-subtask-\d+\.md)\)", subtask_title)
            if plan_match:
                result["has_plan"] = True
                result["plan_file"] = plan_match.group(1)

            break

    return result


def main():
    try:
        input_data = json.load(sys.stdin)
    except json.JSONDecodeError:
        sys.exit(0)

    # Only run on actual session start, not resume/compact
    source = input_data.get("source", "")
    if source not in ("startup", ""):
        sys.exit(0)

    project_dir = os.environ.get("CLAUDE_PROJECT_DIR", os.getcwd())

    # Read CLAUDE.md
    claude_md_path = os.path.join(project_dir, "CLAUDE.md")
    if not os.path.exists(claude_md_path):
        sys.exit(0)

    with open(claude_md_path, encoding="utf-8") as f:
        claude_md_content = f.read()

    # Find active feature
    active_feature = find_active_feature(claude_md_content)
    if not active_feature:
        sys.exit(0)

    # Read feature README
    feature_readme_path = os.path.join(project_dir, active_feature, "README.md")
    if not os.path.exists(feature_readme_path):
        sys.exit(0)

    with open(feature_readme_path, encoding="utf-8") as f:
        readme_content = f.read()

    # Parse and display status
    info = parse_feature_readme(readme_content)

    # Build status message
    lines = [
        "━" * 50,
        f"📋 {info['name'] or 'Unknown Feature'}",
        f"   Status: {info['status'] or 'Unknown'}",
    ]

    if info["current_subtask"]:
        # Clean up subtask title (remove plan link for display)
        subtask_display = re.sub(r"\s*—?\s*\[plan-subtask-\d+\.md\].*", "", info["current_subtask"])
        lines.append(f"   Current: Subtask {info['current_subtask_num']} — {subtask_display}")

        if info["has_plan"]:
            lines.append(f"   Plan: ✅ {info['plan_file']}")
        else:
            lines.append(f"   Plan: ❌ Missing (create plan-subtask-{info['current_subtask_num']}.md)")
    else:
        lines.append("   Current: All subtasks completed! 🎉")

    lines.append("━" * 50)

    # Output as system message
    output = {"systemMessage": "\n".join(lines)}
    print(json.dumps(output))

    sys.exit(0)


if __name__ == "__main__":
    main()
