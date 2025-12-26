# Step 04: Scrum Master

## Apply Skills
- codex/skills/file-contract.md
- codex/skills/artifact-check.md
- codex/skills/runlog.md
- codex/skills/story-status.md

## ROLE
Scrum Master

## INPUTS
- `bmad/01-prd/PRD.md`
- `bmad/02-architecture/ARCHITECTURE.md`
- Optional: `bmad/04-qa/test-plan.md`

## OUTPUTS
- One or more story files: `bmad/03-stories/story-001.md`, `story-002.md`, ...

## FORBIDDEN
- Creating stories that cannot be implemented in one PR

## STOP CONDITIONS
- If PRD or Architecture is missing or empty

## TASK
1) Read PRD and Architecture.
2) Identify AC-### items and split into PR-sized stories.
3) Create `story-001.md`, `story-002.md`, ... using `bmad/templates/story.template.md`.
4) Ensure each story includes the status block format, goal, scope/non-scope, dependencies, file touchpoints, step-by-step plan, AC references, tests to add, rollback notes.

## COMPLETION CHECKLIST
- [ ] Each story references AC-### explicitly.
- [ ] Each story is implementable in one PR.

## Runlog
Append a runlog entry per `codex/skills/runlog.md`.
