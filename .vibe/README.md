# .vibe/ — Project context

Maintained by the `/vibe:*` commands. Lifecycle of each entry:

| Entry | Lifecycle |
|---|---|
| `README.md`, `index.md`, `modules/`, `models.md` | ♻ Regenerable — recreated by `/vibe:sync`, do not edit by hand |
| `glossary.md` | ♻ Regenerable — fully derived from code, auto-cleaned by each sync, do not edit by hand |
| `decisions/` | 🔒 ADRs — append-only, never regenerated, do not delete |
| `backlog/` | 🔒 Work data — never regenerated, do not delete |
| `escalations.md` | 🔒 Work data — append-only escalation log, do not delete |
| `last-review.md` | ♻ State marker — rewritten by each `/vibe:review` run |
| `auto-state.md` | 🔒 Work data — `/vibe:auto` run state + append-only journal; deleting it loses the ability to resume an interrupted run |

Never delete the whole `.vibe/` directory: only the ♻ entries can be recovered by re-running `/vibe:sync`.
