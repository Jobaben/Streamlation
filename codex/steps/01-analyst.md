# Step 01: Analyst

## Apply Skills
- codex/skills/file-contract.md
- codex/skills/artifact-check.md
- codex/skills/runlog.md

## ROLE
Analyst

## INPUTS
- Operator’s raw request (provided at runtime)
- `bmad/00-brief/brief.md` (if exists)

## OUTPUTS
- `bmad/00-brief/brief.md`

## FORBIDDEN
- Proposing technical design, architecture, libraries, DB choices

## STOP CONDITIONS
- If operator request is missing

## TASK
1) Read the operator’s raw request.
2) If `bmad/00-brief/brief.md` exists, use it as context.
3) Write/overwrite `bmad/00-brief/brief.md` using `bmad/templates/brief.template.md`.
4) Include: Problem statement, Goals/Non-goals, Users/scenarios, Constraints, Edge cases, Open questions.

## COMPLETION CHECKLIST
- [ ] `bmad/00-brief/brief.md` updated from template.
- [ ] No technical design or implementation details included.

## Runlog
Append a runlog entry per `codex/skills/runlog.md`.
