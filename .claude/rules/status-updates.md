# Status Section Updates

## When to Update README Status

The `# Status` section in README.md is a brief paragraph describing the current project focus and direction. Update it during **strategic moments**, not routine work.

### Update When:
- Switching active feature (e.g., finishing Kafka, starting Observability)
- Adding new direction or track (e.g., adding Security Foundations to roadmap)
- Completing a major milestone
- Changing priorities or pausing work
- Project focus shifts based on external factors (job requirements, learning goals)

### Don't Update When:
- Completing individual subtasks within a feature
- Fixing bugs or tech debt
- Minor refactoring
- Adding tests or documentation

## Format

Keep it to 2-4 sentences. Include:
1. What's currently in progress
2. Why (context/motivation if relevant)
3. What's next (optional)

### Example:
```markdown
# Status

Currently building **Observability** infrastructure — Prometheus metrics, Grafana dashboards,
and distributed tracing. This is a prerequisite for meaningful benchmarks in the paused
Inter-Service Communication feature. Next focus will be **Security Foundations** to align
with miltech job requirements.
```

## Location

- Status paragraph: `README.md` → `# Status` section
- Detailed roadmap: `docs/roadmap.md`
- Active feature details: `docs/features/{feature}/README.md`
