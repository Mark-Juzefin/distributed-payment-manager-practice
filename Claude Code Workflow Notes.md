# Claude Code Workflow

Notes on using Claude Code as a learning accelerator for this project.

## Core Idea


## Workflow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         SESSION START                                    │
│  Hook автоматично показує: активну фічу, поточний subtask, статус плану │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │  План для subtask існує?      │
                    └───────────────────────────────┘
                           │              │
                          NO             YES
                           │              │
                           ▼              ▼
              ┌────────────────────┐  ┌────────────────────┐
              │  PLANNING PHASE    │  │  IMPLEMENTATION    │
              │  - Обговорення     │  │  - Код по плану    │
              │  - Trade-offs      │  │  - Чекбокси        │
              │  - Зберегти план   │  │  - Тести (manual)  │
              └────────────────────┘  └────────────────────┘
                           │              │
                           └──────┬───────┘
                                  ▼
                    ┌───────────────────────────────┐
                    │  Subtask завершено?           │
                    │  → Mark checkbox ✅            │
                    │  → Next subtask               │
                    └───────────────────────────────┘
                                  │
                                  ▼
                    ┌───────────────────────────────┐
                    │  Feature завершена?           │
                    │  → Update roadmap             │
                    │  → Link next feature          │
                    └───────────────────────────────┘
```

## File Structure

```
CLAUDE.md                           ← Entry point, links to active feature
docs/
├── roadmap.md                      ← High-level feature list with status
└── features/
    └── {NNN}-{feature-name}/
        ├── README.md               ← Feature spec, subtasks, checkboxes
        ├── plan-subtask-1.md       ← Detailed plan for subtask 1
        ├── plan-subtask-2.md       ← Detailed plan for subtask 2
        └── notes.md                ← Optional session notes

.claude/
├── settings.json                   ← Hook configuration
├── hooks/
│   ├── session-start.py            ← Shows feature status on start
│   └── validate-plan-location.py   ← Blocks plans outside feature folders
├── commands/
│   ├── feature-status.md           ← /feature-status dashboard
│   ├── new-feature.md              ← /new-feature [name] [desc]
│   └── pr-summary.md               ← /pr-summary for squash merges
└── rules/
    ├── feature-planning.md         ← Plan file naming rules
    ├── migrations.md               ← DB migration test requirements
    └── status-updates.md           ← When to update README Status
```

## Hooks

### SessionStart: Feature Status Display
При старті сесії автоматично показує:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Feature 003: Inter-Service Communication
   Status: In Progress
   Current: Subtask 1 — HTTP Sync Mode
   Plan: ✅ plan-subtask-1.md
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### PreToolUse: Plan Location Validator
Блокує збереження `plan-subtask-*.md` файлів поза папками фіч:
```
❌ План має бути в папці фічі!

   Неправильно: plan-subtask-1.md
   Правильно:   docs/features/{feature-folder}/plan-subtask-N.md
```

## Slash Commands

| Command | Purpose |
|---------|---------|
| `/feature-status` | Progress dashboard across all features |
| `/new-feature [name] [desc]` | Create feature folder with README template |
| `/pr-summary` | Generate PR title + description for squash merge |

## Typical Session Scenarios

### Scenario 1: Continue Implementation
```
1. Start session
2. Hook shows: "Subtask 2, Plan: ✅"
3. Read plan-subtask-2.md
4. Continue from last checkbox
5. Mark completed tasks
```

### Scenario 2: Start New Subtask
```
1. Start session
2. Hook shows: "Subtask 3, Plan: ❌ Missing"
3. Discuss approach with Claude
4. Approve plan → Claude saves plan-subtask-3.md
5. Begin implementation
```

### Scenario 3: Feature Complete
```
1. All checkboxes done
2. Claude prompts: "Фіча завершена!"
3. Update feature status → Done
4. Update roadmap.md
5. Link next feature in CLAUDE.md
```

## Rules Enforced

### Feature Planning (`.claude/rules/feature-planning.md`)
- Plans must be in feature folders
- Naming: `plan-subtask-N.md`
- README.md stays clean (only status + checkboxes)

### Migrations (`.claude/rules/migrations.md`)
- Every schema change needs integration tests
- Unique constraints must be tested
- Partitioned tables: include partition key in constraints

### Status Updates (`.claude/rules/status-updates.md`)
- Update README Status on strategic changes (new direction, milestone, focus shift)
- Don't update on routine work (subtasks, bugs, refactoring)
- Keep it to 2-4 sentences with context