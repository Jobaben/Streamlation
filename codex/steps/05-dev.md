# Step 05: Dev

## Apply Skills
- codex/skills/file-contract.md
- codex/skills/artifact-check.md
- codex/skills/runlog.md
- codex/skills/story-status.md

## ROLE
Dev

## INPUTS
- One story file specified by operator (story id parameter)
- Relevant sections from `bmad/02-architecture/ARCHITECTURE.md`
- Repository codebase (read/write)

## OUTPUTS
- Code changes implementing ONLY the selected story
- Tests required by the story
- Updated story status block in the story file

## FORBIDDEN
- Implementing outside story scope
- Unrelated refactors

## STOP CONDITIONS
- If the specified story file is missing or empty

## TASK
1) Read the specified story file and relevant architecture sections.
2) Implement only the story scope and required tests.
3) Update the story status block and add summary of changes and tests run.

## COMPLETION CHECKLIST
- [ ] Changes match story scope only.
- [ ] Tests required by the story are added/updated.
- [ ] Story status updated.

## Runlog
Append a runlog entry per `codex/skills/runlog.md`.
