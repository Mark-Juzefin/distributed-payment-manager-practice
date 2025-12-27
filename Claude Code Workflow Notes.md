# Claude Code Workflow

Notes on using Claude Code as a learning accelerator for this project.

## Setup

The project uses three key files to maintain context across sessions:

- **CLAUDE.md** - Instructions for Claude + link to active feature
- **docs/features/*.md** - Detailed spec for each feature with task checkboxes
- **docs/roadmap.md** - High-level plan linking to feature files

## Session Flow

1. Start new session for each feature
2. Claude reads CLAUDE.md → sees active feature → loads context automatically
3. Discuss architecture, create implementation plan
4. Claude implements, I run tests and fix issues myself (learning by doing)
5. When feature complete → Claude updates all files and links to next feature

## Why This Works

- No copy-pasting context between sessions
- Progress tracked via checkboxes in feature files
- Claude explains trade-offs but doesn't auto-fix my mistakes
- I learn by debugging, Claude accelerates the boring parts
