# Feature Planning Rules

## Folder Structure

Each feature has its own folder:

```
docs/features/
├── 001-kafka-ingestion/
│   ├── README.md           ← main feature file (status, overview, tasks)
│   ├── plan-subtask-1.md   ← plan for subtask 1
│   ├── plan-subtask-2.md   ← plan for subtask 2
│   └── ...
├── 002-sharding/
│   ├── README.md
│   └── ...
```

## Naming Conventions

| File | Purpose |
|------|---------|
| `README.md` | Main feature file - status, overview, task checkboxes, notes |
| `plan-subtask-N.md` | Detailed implementation plan for subtask N |

## Planning Workflow

### Before Creating a Plan

1. Read feature's `README.md`
2. Find current subtask (first unchecked task)
3. Check if plan already exists for that subtask
4. If plan exists - read it and continue implementation
5. If no plan - start planning phase

### Creating a Plan

1. Discuss approach with user, explain trade-offs and alternatives
2. Ask clarifying questions if needed
3. When user approves - save plan to feature folder as `plan-subtask-N.md`
4. Add link to README.md

### Plan File Structure

```markdown
# План: {Subtask Name}

## Мета
Brief goal description

## Поточний стан
What exists now

## Архітектурні рішення
| Питання | Рішення | Чому |

## Структура пакетів
Package layout (if applicable)

## Імплементація
Detailed implementation steps with code examples

## Файли для модифікації
| Файл | Зміни |

## Порядок імплементації
Numbered steps
```

### After Plan is Saved

Add link to feature's README.md:

```markdown
**Subtask N план:** [plan-subtask-N.md](plan-subtask-N.md)
```

## Important Rules

- NEVER save plans outside the feature folder
- NEVER use generic names like `plan.md`
- Each subtask gets its own plan file
- README.md stays clean - only status, overview, tasks, and notes
- Plans contain all implementation details
