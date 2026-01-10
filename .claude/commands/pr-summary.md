---
description: Generate PR title and description for squash merge
allowed-tools: Bash
---

# PR Summary Generator

Generates a PR title and description optimized for squash merge.

## Task

1. **Get branch info:**
   ```bash
   git branch --show-current
   git log main..HEAD --oneline
   ```

2. **Analyze commits** to understand the scope of changes:
   - Group by type (feat, fix, refactor, docs, etc.)
   - Identify the main theme/goal

3. **Generate PR title:**
   - Format: `type: short description`
   - Should summarize ALL commits, not just the last one
   - Max 72 characters
   - Examples:
     - `feat: add Kafka-based webhook ingestion`
     - `refactor: extract Ingest service from monolith`

4. **Generate PR description:**
   ```markdown
   ## Summary

   Brief 1-2 sentence overview of what this PR does.

   ## Changes

   - Bullet points of main changes
   - Grouped logically (not 1:1 with commits)
   - Focus on "what" and "why", not "how"

   ## Notes (optional)

   Any important context, breaking changes, or follow-up items.
   ```

5. **Output format:**
   ```
   📝 PR Summary for Squash Merge
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

   **Title:**
   {generated title}

   **Description:**
   {generated description}

   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   ```

## Usage

```bash
# Generate PR summary for current branch
/pr-summary
```

## Notes

- Optimized for squash merge (single commit message)
- Analyzes ALL commits in the branch, not just recent
- Groups related changes for cleaner description
- Language: English only (for commits and PR descriptions)
