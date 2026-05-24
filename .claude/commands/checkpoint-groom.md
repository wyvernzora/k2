---
description: Groom the current topic's checkpoint note under .checkpoint/ (always runs in an Opus subagent).
---

Invoke the `checkpoint-groom` skill. Identify the target topic from
recent work or by listing `.checkpoint/`.

**This skill always dispatches to an Opus subagent.** Per the skill's
dispatch rule, do NOT perform the grooming inline. Spawn a
general-purpose subagent with `model: "opus"` and the prompt template
defined in
`.claude/skills/checkpoint-groom/SKILL.md` (fill in `<TOPIC>` and
`<ABSOLUTE_PROJECT_ROOT>`), wait for it to finish, then relay its
summary back to the user.

The subagent walks the six operations defined in the skill (drain
inbox → drop irrelevant → compress superseded → de-duplicate →
re-evaluate outstanding → promote met REVISIT conditions),
conservative by default, checking the code at HEAD before trusting
memory about what got done. Op 0 (drain inbox) is usually the bulk of
the work now that `checkpoint-notes` defaults to cheap append-only
capture.

This is **not** Claude Code's `/compact` (which summarizes the
conversation transcript). This grooms a checkpoint file on disk.

If multiple topics need attention, do them one at a time (separate
subagent invocations).
