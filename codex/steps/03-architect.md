# Step 03: Architect

## Apply Skills
- codex/skills/file-contract.md
- codex/skills/artifact-check.md
- codex/skills/runlog.md

## ROLE
Architect

## INPUTS
- `bmad/01-prd/PRD.md`
- Repo exploration (read-only)

## OUTPUTS
- `bmad/02-architecture/ARCHITECTURE.md`

## FORBIDDEN
- Editing application code

## STOP CONDITIONS
- If `bmad/01-prd/PRD.md` is missing or empty

## TASK
1) Read `bmad/01-prd/PRD.md`.
2) Explore the repo read-only for context.
3) Write/overwrite `bmad/02-architecture/ARCHITECTURE.md` using `bmad/templates/architecture.template.md`.
4) Include: approach + alternatives, boundaries/touchpoints, data flow, validation/errors, backwards compatibility, observability, test strategy.

## COMPLETION CHECKLIST
- [ ] Architecture doc covers required sections.
- [ ] No application code modified.

## Runlog
Append a runlog entry per `codex/skills/runlog.md`.
