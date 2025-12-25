# BMAD Artifact Map

This directory contains the deterministic, file-driven BMAD workflow artifacts.

## Artifact Map
- `00-brief/brief.md` — Analyst brief
- `01-prd/PRD.md` — Product requirements
- `02-architecture/ARCHITECTURE.md` — Architecture decisions
- `03-stories/` — Story files
- `04-qa/` — QA outputs (test plan, risk register, reviews)
- `05-runlogs/` — Run logs per session
- `templates/` — Canonical templates for all artifacts

## Role Contracts (File-Driven)

### Analyst
**INPUTS (read):**
- Operator’s raw request (provided when running the step)
- `bmad/00-brief/brief.md` (if exists)

**OUTPUTS (write/overwrite):**
- `bmad/00-brief/brief.md`

**MUST INCLUDE:**
- Problem statement
- Goals / Non-goals
- Users & scenarios
- Constraints (security, privacy, compatibility)
- Edge cases / failure modes
- Open questions

**MUST NOT:**
- Propose technical design, architecture, libraries, DB choices

### PM
**INPUTS:**
- `bmad/00-brief/brief.md`

**OUTPUTS:**
- `bmad/01-prd/PRD.md`

**MUST INCLUDE:**
- Overview
- Personas / journeys
- Functional requirements (FR-###)
- Non-functional requirements (NFR-###)
- Acceptance criteria (AC-###) mapped to FRs
- Out of scope
- Rollout plan (optional)
- Metrics/telemetry (optional)

**MUST NOT:**
- Include implementation details (no class names, DB schema, libraries)

### Architect
**INPUTS:**
- `bmad/01-prd/PRD.md`
- Repo exploration (read-only)

**OUTPUTS:**
- `bmad/02-architecture/ARCHITECTURE.md`

**MUST INCLUDE:**
- Proposed approach + alternatives considered
- System boundaries / touchpoints (where changes occur)
- Data flow narrative
- Validation + error handling
- Backwards compatibility / migration
- Observability (logs/metrics)
- Test strategy outline (unit/integration/e2e)

### Scrum Master
**INPUTS:**
- `bmad/01-prd/PRD.md`
- `bmad/02-architecture/ARCHITECTURE.md`
- Optional: `bmad/04-qa/test-plan.md`

**OUTPUTS:**
- One or more stories: `bmad/03-stories/story-001.md`, `story-002.md`, ...

**EACH STORY MUST INCLUDE:**
- Status block (see story-status skill)
- Goal
- Scope / non-scope
- Dependencies
- File touchpoints (expected files/modules)
- Step-by-step implementation plan (detailed)
- Acceptance criteria references (AC-###)
- Tests to add (mapped to test plan)
- Rollback notes if relevant

**HARD RULE:**
- Each story must be implementable in one PR; split if not

### Dev
**INPUTS:**
- Exactly one story file (selected by parameter)
- Relevant sections from `ARCHITECTURE.md`
- Repository codebase (read/write)

**OUTPUTS:**
- Code changes implementing ONLY that story
- Tests required by that story
- Update story status block in the story file

**HARD RULES:**
- Implement story scope only
- No unrelated refactors
- Add/modify tests as specified
- After implementation: summarize changes and list test commands to run

### QA
**INPUTS:**
- Story file
- PRD + ARCHITECTURE
- Current diff (read-only)

**OUTPUTS:**
- `bmad/04-qa/review-story-00X.md`
- Optionally update `test-plan` / `risk-register` if new issues discovered

**MUST INCLUDE:**
- Verdict (Pass / Needs Fix)
- Deviations from story/PRD/architecture
- Missing tests
- Risks and security/privacy checks
- Ordered fix list

### Runlog (cross-cutting)
After each step execution, append an entry to:
- `bmad/05-runlogs/session-YYYY-MM-DD.md`

**Entry must include:**
- Step executed
- Files created/updated
- Key decisions
- Open questions
- Next step recommendation
