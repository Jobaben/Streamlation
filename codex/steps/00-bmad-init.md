# Step 00: BMAD Init

## Apply Skills
- codex/skills/file-contract.md
- codex/skills/runlog.md

## ROLE
Initializer

## INPUTS
- Operator invocation of this step

## OUTPUTS
- bmad/ (entire scaffold and templates)
- codex/ (skills, steps, README)

## FORBIDDEN
- Modifying application code outside of BMAD/Codex scaffolding

## STOP CONDITIONS
- If file system is read-only

## TASK
1) Create or repair `bmad/` structure and template files.
2) Create or repair `codex/` structure with skills, steps, and README.
3) Do not modify application logic.

## COMPLETION CHECKLIST
- [ ] `bmad/` structure exists with required files.
- [ ] `codex/` structure exists with required files.
- [ ] No application logic files modified.

## Runlog
Append a runlog entry per `codex/skills/runlog.md`.
