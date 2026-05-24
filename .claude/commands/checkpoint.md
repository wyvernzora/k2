---
description: Refresh the current topic's checkpoint note under .checkpoint/.
---

Invoke the `checkpoint-notes` skill. Identify the current topic from
recent work (default: match an existing file under `.checkpoint/`).

**Default path is cheap capture:** append tagged lines to `## Inbox` at
the top of `<project>/.checkpoint/<topic>.md`, bump `Updated:`, and
stop. Do not reorganize — that's the `checkpoint-groom` skill's job.

Caveman-ultra style; one fact per line. Bias toward over-capture; the
inbox cost is one line per entry. Use the tag set from the skill
(`TODO`, `DECISION`, `LEARNING`, `CONSTRAINT`, `QUESTION`, `RISK`,
`REVISIT(...)`, `SUPERSEDED`).

Full-update path (read whole file, rewrite Resume, drain inbox via
`checkpoint-groom`) only when explicitly handing off or when the
inbox has bloated past skimmability.

If no existing topic fits, ask before creating a new one.
