# Claude Code Workflow

Notes on using Claude Code as a learning accelerator for this project.

## Setup

The project uses these files to maintain context across sessions:

- **CLAUDE.md** - Instructions for Claude + link to active feature
- **docs/features/{feature}/README.md** - Feature spec with task checkboxes
- **docs/features/{feature}/plan-subtask-N.md** - Implementation plans per subtask
- **docs/roadmap.md** - High-level plan linking to feature folders
- **.claude/rules/** - Enforced rules (migrations, feature planning)

## Session Flow

1. Start new session
2. Claude reads CLAUDE.md → active feature folder → README.md
3. Finds current subtask (first unchecked), checks if plan exists
4. If no plan → discuss approach → save to `plan-subtask-N.md`
5. If plan exists → implement following the plan
6. When feature complete → Claude updates files and links to next feature