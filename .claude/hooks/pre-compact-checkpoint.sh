#!/usr/bin/env bash
# Reminder injected before context compaction. The model still does the
# writing — this just makes sure it doesn't forget. See:
#   .claude/skills/checkpoint-notes/SKILL.md
set -euo pipefail

cat <<'EOF'
{
  "hookSpecificOutput": {
    "hookEventName": "PreCompact",
    "additionalContext": "Context compaction is about to fire. Before continuing, invoke the checkpoint-notes skill and refresh the relevant .checkpoint/<topic>.md so operational state survives. Use caveman-ultra style; do not include reasoning chains."
  }
}
EOF
