# Codex BMAD Control Plane

## How to Run (BMAD on Codex CLI)

1) Initialize scaffolding (idempotent):
```
codex run codex/steps/00-bmad-init.md
```

2) Create brief (Analyst):
```
codex run codex/steps/01-analyst.md
```
- Provide the raw request when prompted or inline.

3) Create PRD (PM):
```
codex run codex/steps/02-pm.md
```

4) Create Architecture (Architect):
```
codex run codex/steps/03-architect.md
```

5) Generate Stories (Scrum Master):
```
codex run codex/steps/04-scrum.md
```

6) Implement Story 001 (Dev):
```
codex run codex/steps/05-dev.md
```
- Provide parameter story id: 001

7) Review Story 001 (QA):
```
codex run codex/steps/06-qa.md
```
- Provide parameter story id: 001

8) Apply fixes and re-run QA until Pass, then proceed to story 002.

## Rules of Engagement
- PRD contains NO implementation details
- Architecture contains technical approach
- Stories are single-PR units
- Dev implements one story at a time
- QA enforces alignment and test coverage
- All state is persisted in bmad/ artifacts
- Runlog is appended on every step

## Example: Full Walkthrough
**Request:** “Add CSV export for the report page.”

1) **Init**
   - Creates: `bmad/` and `codex/` scaffolding, templates, and README files.

2) **Analyst**
   - Writes: `bmad/00-brief/brief.md` with problem, goals, constraints, and open questions.

3) **PM**
   - Writes: `bmad/01-prd/PRD.md` with FR/NFR and AC mapping.

4) **Architect**
   - Writes: `bmad/02-architecture/ARCHITECTURE.md` with approach and test strategy.

5) **Scrum Master**
   - Writes: `bmad/03-stories/story-001.md` (and more if needed).
   - Story status starts at `Draft` and is advanced to `Ready` when approved.

6) **Dev**
   - Implements story-001.
   - Updates status to `In Review` with PR placeholder and tests listed.

7) **QA**
   - Writes: `bmad/04-qa/review-story-001.md` with verdict and fix list.
   - If Pass, story moves to `Done`; otherwise fixes are applied and re-reviewed.
