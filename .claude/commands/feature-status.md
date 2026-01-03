---
description: Show current feature progress, task completion, and active subtask
allowed-tools: Read, Glob
---

# Feature Status Dashboard

Shows the current state of all features with focus on the active one.

## Task

1. **Read CLAUDE.md** to find active feature path

2. **Scan all features** in `docs/features/*/README.md`

3. **For each feature, extract:**
   - Feature name (from heading)
   - Status (Planned/In Progress/Done)
   - Task list with completion count
   - Whether plan files exist (`plan-subtask-*.md`)

4. **Display summary table:**
   ```
   📊 Feature Status Dashboard

   Active Feature: {name from CLAUDE.md}
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

   | Feature | Status | Progress | Plans | Current Subtask |
   |---------|--------|----------|-------|-----------------|
   | 001-kafka-ingestion | Done | 5/5 ✅ | Yes | - |
   | 002-ingest-extraction | In Progress | 2/4 ⏳ | Yes | Subtask 3: [name] |
   | 003-sharding | Planned | 0/6 📋 | No | - |
   ```

5. **For active feature, show details:**
   - Which subtask is current (first unchecked)
   - Does plan exist for current subtask?
   - What's blocking progress (if mentioned in Notes)

6. **Actionable next steps:**
   - If no plan for current subtask: "▶️ Next: Create plan for subtask X"
   - If plan exists: "▶️ Next: Continue implementation from plan-subtask-X.md"
   - If all tasks done: "✅ Feature complete! Mark as Done and move to next?"

## Example Output

```
📊 Feature Status Dashboard
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎯 Active: 002-ingest-service-extraction (In Progress)

Progress: 2/4 tasks completed (50%)

✅ Subtask 1: Extract Ingest Service
✅ Subtask 2: Configure Kafka mode
⏳ Subtask 3: Add integration tests ← CURRENT
☐ Subtask 4: Update documentation

Plan status: plan-subtask-3.md exists ✓

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Other Features:

001-kafka-ingestion     | Done        | 5/5 ✅
003-distributed-tracing | Planned     | 0/6 📋

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
▶️ Next Step: Continue implementing subtask 3 using plan-subtask-3.md
```

## Usage

```bash
# Show feature status
/feature-status
```

## Notes

- Highlights active feature from CLAUDE.md
- Shows clear visual distinction between done/in-progress/planned
- Identifies exact current subtask to avoid confusion
- Reminds about plan files for systematic work
