---
description: Create new feature folder with README template and update roadmap
argument-hint: [feature-name] [feature-description]
---

# Create New Feature Structure

Creates a complete feature folder structure in `docs/features/` with proper templates and updates project tracking files.

## Task

1. **Find next feature number**
   - Look in `docs/features/` for highest number
   - Increment by 1 for new feature

2. **Create folder structure**
   ```
   docs/features/{number}-{feature-name}/
   └── README.md
   ```

3. **Generate README.md** with this template:
   ```markdown
   # Feature: {Feature Name}

   **Status:** Planned

   ## Overview
   {feature-description from argument}

   ## Implementation Plan
   Plan files will be linked here as subtasks are broken down.

   ## Tasks
   - [ ] Subtask 1: [Ask user to define tasks]
   - [ ] Subtask 2:
   - [ ] Subtask 3:

   ## Architecture Notes
   (To be filled during planning)

   ## Testing Strategy
   (To be filled during planning)

   ## Notes
   - Created: {current date}
   ```

4. **Ask user:**
   - What specific subtasks should be included?
   - Any architectural considerations to note upfront?

5. **Update CLAUDE.md:**
   - Change `Active feature:` line to point to new feature folder

6. **Remind user:**
   - "Не забудь оновити docs/roadmap.md з новою фічею!"
   - Show what the roadmap entry should look like

## Example Usage

```bash
/new-feature distributed-tracing "Add OpenTelemetry for distributed tracing across services"
```

## Expected Output

- Creates `docs/features/003-distributed-tracing/README.md`
- Updates CLAUDE.md active feature link
- Prompts for task breakdown
- Reminds about roadmap.md update
