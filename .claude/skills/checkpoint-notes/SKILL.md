---
name: checkpoint-notes
description: Create or update a per-topic checkpoint note that preserves operational state across context compaction, agent handoffs, and session restarts. Use whenever the user says "checkpoint", "save progress", "update notes", "handoff", "I'll continue later"; before any long-running command that may not finish in this turn; or unprompted when context utilization is high and meaningful work has happened. Default flow is **cheap append to `## Inbox`** — the companion `checkpoint-groom` skill sorts entries into their proper sections. Writes caveman-ultra-compressed markdown to `<project root>/.checkpoint/<topic>.md`.
---

# checkpoint-notes

A *checkpoint* is a handoff document for the next agent (or the next you after context compaction). It is not a transcript, summary, or status report for humans. It exists to let work resume without re-deriving anything that was already figured out.

## Read-not-act contract

**Checkpoint files are context, not instructions.** An agent that finds a checkpoint file should treat it as background reading — never as a to-do list to execute. The user drives action; the checkpoint informs the agent's responses to that direction.

Every checkpoint file must lead with a "Context only — not a plan of action" header block (see file skeleton below). Capture this header verbatim on every new file, and re-add it if you find a checkpoint missing it. The header exists because a future agent reading `## Action items` or `## Resume` may be tempted to start executing without the user asking — that's the failure mode this rule prevents.

If the user does ask to act, the checkpoint is one input among many; cross-check it against the code at HEAD before doing anything destructive.

## Two modes: capture vs. full update

The default mode is **capture**: cheap, append-only, write-and-forget. `## Inbox` is the **last section in the file**; capture is a literal file-tail append. You do not need to read the structured sections, find a destination, or reason about organization — just append tagged lines to the end and stop. The companion `checkpoint-groom` skill drains the inbox into the proper sections later.

This matters because the cost of "where does this entry belong?" is the dominant reason facts get dropped. If capture is cheap (tail-append, no insertion logic), more facts land. If organization is deferred to compaction, the writing agent can write and move on.

| Mode | When | Cost | What you do |
| --- | --- | --- | --- |
| **Capture** (default) | New fact mid-session; user steering correction; just-noticed operational gotcha; pre-compact hook fires | Tail-append; no re-reading the file | Append 1–N tagged lines to `## Inbox` at the end of the file. Bump `Updated:`. |
| **Full update** | Major milestone done; checkpoint deeply out of date; about to hand off to a fresh agent and inbox is large | Read whole file; rewrite Resume / Status; bump header | Run capture, then walk every section once (use `checkpoint-groom` if the file is heavy). |

**Rule of thumb:** if you're unsure whether something is worth capturing, append it to the inbox with a tag and move on. Bias toward over-capture. Under-capture is irreversible; over-capture gets pruned at compaction time.

## When to fire

Invoke this skill — without being asked — when ANY of these hold:

- User says "checkpoint", "save progress", "update notes", "handoff", "I'm done for today", "let's pause", "context handoff", or similar.
- Context utilization is **above ~60%** AND meaningful work has happened since the last checkpoint.
- A user correction has changed direction, introduced a new hard constraint, or invalidated a prior decision. Capture it immediately while the framing is fresh.
- The pre-compact hook fires (see "Hook wiring" below).
- About to start a long-running operation that the current turn may not complete.
- You notice **any** of the easy-to-miss categories listed in "What to capture (easy to miss)" below. Append to inbox; don't gate on whether it "deserves" a checkpoint.

Do **not** fire for: trivial single-question turns, read-only exploration, or work that left no operational state behind.

## What to capture (easy to miss)

These categories are routinely under-captured because they don't feel like "today's work". Capture them anyway. All of them are inbox-append candidates — no need to figure out the destination section.

1. **Forward-references: hard-coded values in code that will need treatment when their owning component lands.** Example: `K2Issuer email + AWS region currently hard-coded in legacy cert-manager; move to cluster YAML or context at port time.` Tag with `REVISIT(<that-component>-port)` so it surfaces when the port starts.
2. **Operational prereqs tied to a not-yet-built component.** Example: `Longhorn must exclude control-plane nodes via its own chart config (defaultNodeSelector) before any K2Volume.replicated works.` Tag `LEARNING:` if generalizable, or `RISK:` if it's a "will bite us" item.
3. **Non-obvious behavioral assumptions of dependencies.** Things the code doesn't make obvious that an agent will re-derive incorrectly. Example: `cdk8s-plus auto-attaches volumes referenced by mounts, so SimpleMaterializedVolume.configureWorkload is a no-op.` Tag `LEARNING:`.
4. **Genuine open questions, distinct from TODOs.** A TODO is "do X". A QUESTION is "we haven't decided whether to do X or Y." Example: `keep Helm strip-CRDs default, or let chart own its CRDs end-to-end?` Tag `QUESTION:`.
5. **Verbatim procedural commands for future operations.** When the cutover / migration / rollback procedure is decided, capture the exact commands, not a paraphrase. The future agent will look for command lines first. Example: `cutover: git push origin --delete main && git branch -m main-v3 main && git push -u origin main`. Tag `REVISIT(<trigger>):` with the literal commands inline.
6. **Discrete sub-items that get compressed into a class.** "Port cert-manager" hides 4 sub-decisions. If you've already made any of them (e.g. "use Route53 DNS01 issuer, not HTTP-01"), capture them as `DECISION:` lines now, not after the port.
7. **The trail of a rejected approach.** When you abandon an approach mid-session, capture `SUPERSEDED:` immediately — by the time it's relevant again, the framing is gone.
8. **User steering corrections.** Anything the user pushed back on hard. Capture as `CONSTRAINT:` (if it's now a rule) or `DECISION:` (if it's a one-off choice). Quote the user's framing if it's pithy.
9. **Anti-default rules — intentional divergences from a pattern the next agent would reflexively reach for.** When the new design intentionally differs from a legacy pattern, prior convention, or a framework default, capture that explicitly. The TypeScript compiler or a runtime error may eventually flag the divergence, but an explicit entry saves the iteration. Example: `CONSTRAINT: createAppResources signature does NOT receive K2SynthContext. App code reads facts via Context.of(this) inside constructs.` (Legacy was `(app, ctx)`; an agent porting from legacy will copy the legacy signature reflexively.) Tag `CONSTRAINT:` (if it's enforced by code) or `DECISION:` (if it's a convention).
10. **The current state behind any open choice.** Whenever you write a QUESTION or RISK, also state what the system does *today*. Without that, the reader can't evaluate the open choice. See "Self-containment" below.

If something feels "too small" to capture: append it anyway. Inbox cost is one line.

## Self-containment

Every entry must be readable in isolation by an agent who has none of your session's context. The reader will likely grep for one tag (`rg '^QUESTION:' .checkpoint/`) and read only the matching lines. If a line is incomprehensible without the rest of the file, the capture failed even if all the bytes are there.

Concrete rules:

- **QUESTION entries must state current behavior, not just the open choice.**
  - ❌ `QUESTION: keep strip-CRDs default, or own them in chart?` *(what's the default today?)*
  - ✅ `QUESTION: HelmCharts.asConstruct currently strips Helm-managed CRDs by default — keep that, or let chart own CRDs end-to-end?`
- **REVISIT entries must describe the action, not just the trigger.**
  - ❌ `REVISIT(traefik landed): §6.8.` *(do what when traefik lands?)*
  - ✅ `REVISIT(traefik landed + @k2/cilium has web preset): §6.8 — add ComponentNetworkPolicy.withWebDefaults({...}) for DNS + apiserver + Git egress + allow-from-traefik.`
- **DECISION entries must capture the rationale, not just the choice.** The next agent considering overturning the decision needs to know what they're trading off.
  - ❌ `DECISION: K2Volume = abstract class + static factories.`
  - ✅ `DECISION: K2Volume = abstract class + late-bound static factories. Was K2VolumeBase + alias + const trio. Cycle via concrete subclasses caused TDZ; late-binding from volumes/index.ts breaks the cycle.`
- **RISK entries must state the trigger + the consequence.**
  - ❌ `RISK: §8.4 #11.` *(what risk?)*
  - ✅ `RISK: +k8s-manifests does NOT auto-run +crd-constructs → chart bump w/ stale TS bindings = silent drift. Workflow doc warns; tools don't enforce.`
- **§/path/sha references must be evidence-backed.** Before writing `§6.7` or `apps/foo/index.ts:42` or `83d3bcc`, verify the section/line/commit actually exists and contains what you're attributing to it. Cite from the source, not from memory. Wrong references waste the next agent's lookup time and erode trust in the file.

The cheap-capture contract does *not* excuse decontextualized entries. Inbox append is fast because you don't have to pick a destination section — it is **not** fast because you can drop context. A self-contained tagged line is the unit of cheap capture; "QUESTION: §6.5?" is not.

## Where checkpoints live

```
<project root>/.checkpoint/<topic>.md
```

- Always relative to the project root (the directory containing `.git`, `package.json`, `Earthfile`, etc.). Resolve it once; don't guess.
- `<topic>` is a short kebab-case identifier (`v3-cdk-design`, `kairos-provisioning`, `cnpg-backup`). Match an existing topic if work continues; create a new file when the topic is materially different.
- One topic per file. Don't multiplex.
- If `.checkpoint/` doesn't exist, create it. Don't commit it unless the repo's `.gitignore` already permits.
- Before creating a new topic file, list `.checkpoint/` and consider whether existing topics already cover the work.

## File shape

Use this skeleton verbatim. Section headers are fixed; keep them even when empty so grep-by-section works. Tag prefixes inside sections (`TODO:`, `DECISION:`, etc.) are the grep handles.

```
# <topic> — <one-line goal>

> **Context only — not a plan of action.** This file is a handoff
> document for the next agent. Do NOT take any action based on its
> contents unless the user explicitly asks you to. Reading is fine;
> acting requires user direction.

Updated: <ISO date>          Branch: <branch>          HEAD: <short sha>

## Goal
<2-3 caveman lines: user's actual goal + most recent steering correction.>

## Status
[x] / [ ] / [~] entries. Latest at top. Mirror these in ## Action items
where applicable.

## Action items
TODO: <next action>
TODO: <...>
BLOCKED: <action> ← <thing blocking>
DOING: <currently active work>

## Constraints
CONSTRAINT: <hard rule from user / AGENTS.md / repo convention>
CONSTRAINT: <...>

## Decisions
DECISION: <choice> b/c <reason>. Ref: <path:line | §x.y | sha>
DECISION: <...>

## Learnings
LEARNING: <correction absorbed this session, generalized into a rule>

## Files & paths
<path>: <why it matters in one line>
<path>: <...>

## Commands
$ <command>                              # <result | exit code | duration>
$ <command>                              # <result>

## Artifacts
<path>: <untracked | gitignored | generated | local-only>. Why it exists.

## Open
QUESTION: <needs user input>
RISK: <potential issue>
REVISIT(<condition>): <thing to fix later, once condition is true>

## Stale
~~SUPERSEDED: <old claim>~~ ← <what replaced it / commit sha>

## Resume
1. <very next concrete command or file to inspect>
2. <next>
3. <next>

## Inbox
<!-- Append-only landing zone for new facts. ALWAYS the last section.
     Each line is a tagged entry. Capture path: open file, jump to end,
     append. Do not reason about destination — that's compaction's job.
     The checkpoint-groom skill drains this into the sections above. -->
TODO: <new action item noticed mid-session>
QUESTION: <open question you don't have time to resolve right now>
LEARNING: <non-obvious behavior you just discovered>
REVISIT(<condition>): <forward-reference; surfaces when condition is met>
SUPERSEDED: <approach you just abandoned>
CONSTRAINT: <new hard rule from user steering>
DECISION: <choice made this session> b/c <reason>. Ref: <path | sha>
```

Section order is fixed. Drop the section if it would be empty AND has never been populated for this topic; keep it (empty) once anything has lived there, so grep finds the heading. The one exception: `## Inbox` should always be present (even when empty) at the **end of the file** so the next agent knows where to tail-append.

## Caveman-ultra style

Optimize for **information density per byte**, not human readability. The reader is another agent that will re-expand context from the source code anyway.

Strict rules:

- Drop articles (`a`, `an`, `the`) unless the sentence becomes ambiguous.
- Drop linking verbs (`is`, `are`, `was`, `were`) where unambiguous. Prefer arrows.
- Use arrows / pipes / colons for relations: `→` (causes / next / replaces), `←` (depends-on / blocked-by), `|` (or / alternative), `+` (and), `~` (approximately).
- File refs: `path/to/file.ts:42` or `path/to/file.ts §heading`. Always relative to project root.
- Commit refs: 7-char SHA prefix.
- Section refs: `§3.1` (the design-doc convention; reuse where it applies).
- Quote real command lines verbatim, prefixed `$`. Never paraphrase.
- Numbers, identifiers, version strings → keep literal.
- Avoid prose paragraphs. One fact per line.

Verbose → caveman examples:

```
Verbose: "We decided to keep K2Volume as an abstract class with static
          factory methods, because the previous K2VolumeBase + type alias
          + const literal arrangement was confusing."
Caveman: DECISION: K2Volume = abstract class + static factories. Was
         K2VolumeBase + alias + const. Confusing.
```

```
Verbose: "The earthly lint step is currently failing because the
          eslint-plugin-import-x rule complains about empty lines
          within import groups when multi-line imports interact with
          path-group config."
Caveman: BLOCKED: `earthly +lint` ← import-x/order empty-line-in-group
         err on multi-line @k2/* import when sibling import present.
         Cause: pathGroup position:after interaction. Workaround:
         remove empty line between groups.
```

## Tag set (grep handles)

Use these tags exactly. They are the grep-able interface to the file. Pick one tag per entry; one entry per line where practical.

| Tag          | Use for                                                       | Self-containment requirement |
| ------------ | ------------------------------------------------------------- | ---------------------------- |
| `TODO:`      | Pending action item. Concrete, actionable, not aspirational. | State the action verb + the object. Not "fix §8.4 #4" — "validate app exports up front (§8.4 #4)". |
| `DOING:`     | Currently in-flight; would resume here on next turn.         | Include the file/branch you'd resume in. |
| `BLOCKED:`   | Pending external action (user, CI, third-party). Note what.  | State the blocker explicitly: `← waiting on <person/thing>`. |
| `DECISION:`  | Choice made. Include rationale. Reference where applied.     | Choice + rationale + reference. Three parts; not optional. |
| `CONSTRAINT:`| Hard rule. Often from AGENTS.md or user steering. No commentary — just state it. | State the rule as an imperative or prohibition. Cite the source (`AGENTS.md §5`, user steering date). |
| `LEARNING:`  | Correction absorbed this session, *generalized*. "Always X for Y", not "today X worked". | Generalize the rule + state the trigger (when does this apply?). |
| `QUESTION:`  | Needs user input. Block on it.                               | **State current behavior** + the open choice. The reader must be able to answer without context. |
| `RISK:`      | Possible failure mode worth tracking; not yet a blocker.     | State the trigger + the consequence. Not just a section reference. |
| `REVISIT(<condition>):` | Suboptimal-by-design choice to fix when `<condition>` becomes true. The condition is the trigger, not a deadline. Examples: `REVISIT(argocd app present with CRDs)`, `REVISIT(traefik landed + web NetPol preset exists)`. | Condition + action. The trigger alone is not enough — say what to do once it fires. **The trigger must match the agent who will actually act on the entry.** One REVISIT per affected work-context; do not bundle cross-cutting forward-refs under one trigger (see failure modes). |
| `SUPERSEDED:`| Old claim still relevant for context but no longer true. Strike through and link to replacement. | Strike-through the old claim + cite the commit/section that replaced it. |

Greps you'll actually use:

```sh
rg '^(TODO|DOING|BLOCKED):' .checkpoint/        # what's left to do
rg '^DECISION:' .checkpoint/                    # decisions made
rg '^CONSTRAINT:' .checkpoint/                  # current hard rules
rg '^LEARNING:' .checkpoint/                    # accumulated corrections
rg '^(QUESTION|RISK):' .checkpoint/             # things that need attention
rg '^REVISIT\(' .checkpoint/                    # deferred work + its trigger
rg '^REVISIT\([^)]*traefik' .checkpoint/        # … gated on a specific condition keyword
```

## Updating vs. creating

Default = update an existing topic. Only create a new file when the new work is genuinely a new topic.

### Capture path (default, cheap)

This is what you do most of the time. It is fast on purpose.

1. **Do not read the file body.** Just confirm the file exists and find the `## Inbox` section at the end. The header date can be bumped without reading the structured sections.
2. **Tail-append tagged lines to `## Inbox`.** One fact per line. Pick a tag from the tag set; do not pick a destination section. The inbox is the last section by design so this is a literal file-tail append.
3. **Bump `Updated:` to today.** Leave `Branch:` and `HEAD:` alone unless they changed.
4. **Stop.** Do not touch other sections. Do not move anything out of inbox. Do not skim sibling sections to "check for duplicates" — that's compaction's job.

If `## Inbox` is missing (older checkpoint pre-dating this skill version), add the section at the **end of the file** before appending. Don't reorganize the rest.

### Full update path (heavier, less frequent)

Use when handing off to a fresh agent, when many things landed at once, or when the inbox is so large it stops being skimmable.

1. **Read the existing file first.** Whole thing.
2. **Bump `Updated:`, `Branch:`, `HEAD:`.**
3. **Append any newly-noticed facts to `## Inbox`** before touching anything else.
4. **Invoke the `checkpoint-groom` skill** to drain the inbox and groom the rest of the file. Don't do that work inline — the companion skill exists so this skill stays cheap.
5. **After compaction returns: re-order `## Action items`** so the next concrete step is first.
6. **Rewrite `## Resume`** to point at the very next thing.

Never:
- Dump the chat transcript.
- Include reasoning chains, agent reflections, "I considered X but…".
- Use lyrical prose. Caveman ultra.
- Add information that's plainly visible in the source code at HEAD.
- **Move entries out of `## Inbox` into other sections during a capture-path update.** That's compaction's job. Doing it inline defeats the whole point of cheap capture.

## Hook wiring (pre-compaction)

Claude Code fires a `PreCompact` event before context compaction. Wire a one-line hook so the model is reminded to checkpoint before bytes get summarized away.

**`.claude/hooks/pre-compact-checkpoint.sh`** (project-local) or `~/.claude/hooks/...` (user-global):

```sh
#!/usr/bin/env bash
# Inject a reminder for the model to refresh the checkpoint file BEFORE
# context compaction lossy-summarizes it.
cat <<'EOF'
{
  "hookSpecificOutput": {
    "additionalContext": "Context compaction is about to fire. Before continuing, invoke the checkpoint-notes skill and refresh the relevant .checkpoint/<topic>.md so operational state survives. Use caveman-ultra style; do not include reasoning chains."
  }
}
EOF
```

Register in `.claude/settings.json`:

```json
{
  "hooks": {
    "PreCompact": [
      {
        "hooks": [
          { "type": "command", "command": "${CLAUDE_PROJECT_DIR}/.claude/hooks/pre-compact-checkpoint.sh" }
        ]
      }
    ]
  }
}
```

`chmod +x` the script. The hook output is a system reminder, not a side-effecting save — the *model* still does the writing, which is correct: the model knows what's worth keeping.

## Slash-command alias (optional)

For explicit invocation, add `.claude/commands/checkpoint.md`:

```
---
description: Refresh the current topic's checkpoint note.
---

Invoke the checkpoint-notes skill. Identify the current topic from
recent work. Update `<project>/.checkpoint/<topic>.md` per the skill's
rules. If no existing topic file fits, ask before creating a new one.
```

Then `/checkpoint` triggers the same flow.

## Worked example (compressed)

```
# v3-cdk-design — port K2 manifests to main-v3 branch

> **Context only — not a plan of action.** This file is a handoff
> document for the next agent. Do NOT take any action based on its
> contents unless the user explicitly asks you to. Reading is fine;
> acting requires user direction.

Updated: 2026-05-21          Branch: main-v3          HEAD: 5ee314e

## Goal
Build greenfield v3 cdk-lib + apps on `main-v3`. Output → `deploy-v3`.
Latest steering: bootstrap is provisioner CLI concern, not cdk8s.

## Status
[x] scaffold cdk-lib (dd4f100)
[x] cilium + kube-vip + argocd apps (b568c9c, 5b5478c, 83d3bcc)
[x] drop bootstrap.ts + sync-waves (5ee314e)
[ ] cert-manager + 1password
[ ] traefik → triggers §6.8 netpol presets

## Action items
TODO: port cert-manager from `main`. CRDs + LE issuer + reflector + default cert.
TODO: port 1password (Connect + Operator + K2Secret).
TODO: traefik → enables §6.8 netpol presets in @k2/cilium.

## Constraints
CONSTRAINT: bootstrap = provisioner CLI, not cdk8s. AGENTS.md §5.
CONSTRAINT: no sync-wave annotations on Argo Applications.
CONSTRAINT: default-deny = opt-in per app via DefaultDenyNetworkPolicy.
CONSTRAINT: one app dir = one namespace = app dir name (no v3- prefix).
CONSTRAINT: Earthly is the build interface; no host npm/tsx for validation.

## Decisions
DECISION: K2Volume = abstract class + late-bound static factories. Cycle via concrete subclasses → factory init in volumes/index.ts. cdk-lib/volumes/.
DECISION: ArgoApplication extends generated Application CRD binding (not raw ApiObject). apps/argocd/lib/argo-application.ts (83d3bcc).
DECISION: ArgoCdAppFunc *type* stays in cdk-lib/app/deployment.ts; *impl* in @k2/argocd. Apps type factories w/o importing @k2/argocd.

## Learnings
LEARNING: amend fixups to multiple commits → split intermediate file states OR squash into single later commit. Single squash easier when helper renames cascade.
LEARNING: import-x/order falsely flags empty-line-in-group on multi-line @k2/* import w/ sibling import present. Workaround: investigate which file actually errors (lint output's "8:1" was misleading).

## Files & paths
notes/v3-cdk-design.md: long-form design doc. .checkpoint/ is operational; this is design.
cdk-lib/cluster/index.ts: hand-rolled validator + exhaustiveness guard.
build/scripts/synthesize-manifests.ts: synth entry; ~118 LOC.
apps/<name>/index.ts: createAppResources + createArgoCdApp typed exports.

## Commands
$ earthly +lint                          # green
$ earthly +k8s-manifests                 # green; deploy/ regenerated clean
$ earthly +crd-constructs                # regen CRD bindings after CRD manifest changes
$ earthly +diff-manifests                # vs remote deploy-v3 branch

## Artifacts
deploy/: generated, gitignored. Synth output.
node_modules/: local dev only. Real builds via Earthly.
.checkpoint/: this directory. Gitignored.

## Open
QUESTION: cert-manager sync-wave? No bootstrap concept → just deploy alongside.
REVISIT(traefik app present + @k2/cilium has web preset): argocd currently has no default-deny CiliumNetworkPolicy. Once traefik is in + preset bundle exists for "DNS + apiserver + Git egress", add `new DefaultDenyNetworkPolicy(...)` to apps/argocd/index.ts.

## Stale
~~SUPERSEDED: bootstrap.ts with BootstrapPolicy map~~ ← removed in 5ee314e per user steering.
~~SUPERSEDED: K2NfsVolume zone-attract hard-required~~ ← changed to weight:100 soft in b568c9c.

## Resume
1. `git checkout main && cat apps/cert-manager/index.ts` — read legacy.
2. `git ls-tree main apps/cert-manager` — list files to port.
3. `git checkout main-v3 && mkdir -p apps/cert-manager/{components,crds,lib}`.
4. Copy Chart.yaml + crds.k8s.yaml from main; `earthly +crd-constructs`.
5. Adapt index.ts to v3 pattern (typed exports, K2Chart, no `ctx`).

## Inbox
QUESTION: Helm strip-CRDs default — keep, or let chart own CRDs end-to-end?
LEARNING: cdk8s-plus auto-attaches volumes referenced by mounts → SimpleMaterializedVolume.configureWorkload is intentionally a no-op.
LEARNING: Longhorn must exclude control-plane via its chart defaultNodeSelector before K2Volume.replicated works.
REVISIT(cert-manager port): legacy K2Issuer email + AWS region + ACME server hard-coded → move to cluster YAML or @k2/cert-manager context.
REVISIT(cutover): exact commands → `git push origin --delete main && git branch -m main-v3 main && git push -u origin main` (repeat for deploy/deploy-v3).
TODO: §8.4 #4 validate app exports up front; print all missing exports at once.
TODO: §8.4 #5 wrap dynamic imports w/ `Error("Failed loading apps/<name>: ...", { cause })`.
```

That whole worked example is ~2 KB. The information content would balloon 10× in human-readable prose.

## Companion skill

When a checkpoint file grows large (~400+ lines) or accumulates many
SUPERSEDED entries, invoke the **`checkpoint-groom`** skill to
consolidate. It walks five operations (drop irrelevant → compress
superseded → dedupe → re-evaluate outstanding → promote met REVISITs)
and shrinks the file without losing operationally-useful context.

The two skills compose: `checkpoint-notes` writes and updates;
`checkpoint-groom` consolidates.

## Failure modes to avoid

- **Editorializing.** "I think we should…" is not a checkpoint. Either it's a `DECISION:` (then state the choice) or it's a `QUESTION:`.
- **Echoing the source code.** Don't list the entire cdk-lib tree. Reference paths and explain *why* they matter, briefly.
- **Stale-by-omission.** When a `DECISION:` is overturned, *move it* to `## Stale` with the SUPERSEDED tag. Never silently delete.
- **Running multiple topics in one file.** If `.checkpoint/v3-cdk-design.md` starts accumulating CNPG / Kairos / unrelated work, split it.
- **Forgetting `Updated:` and `HEAD:`.** Without these the next agent can't tell if the doc is from yesterday or last month.
- **Omitting the "Context only — not a plan of action" header.** Without it the next agent may treat the file as a work order and start executing without user direction. Re-add the header verbatim if you find a file missing it.
- **Decontextualized entries.** A QUESTION that doesn't state current behavior, a REVISIT that only names the trigger, a RISK that's just a `§x.y` pointer — all of these fail self-containment. Capture is cheap *because you skip destination-routing*, not because you skip context.
- **Reference rot.** Citing `§6.7` when the design doc you're referencing has §6.4 / §6.5 / §6.8 but no §6.7 anymore; citing `apps/foo/bar.ts:42` when the file has been refactored; citing a commit SHA that doesn't match what landed. Verify every `§/path/sha` reference against the source the moment you write it.
- **Anti-default rules left implicit.** When the design intentionally inverts a legacy pattern or a framework default, capture the inversion explicitly as a `CONSTRAINT:` or `DECISION:`. "The type signature enforces it" is true but the next agent porting from legacy will copy the legacy signature first and learn from the TS error second. Save them the iteration.
- **REVISIT triggers that bundle cross-cutting forward-refs.** The condition on a `REVISIT(...)` entry is read by the *future* agent doing some specific piece of work — porting cert-manager, porting auth, etc. If you bundle "K2Issuer email move" *and* "Authelia cookie domain move" under `REVISIT(cert-manager port)`, the auth-port agent will never see the Authelia line; the cert-manager-port agent will hit it and not know what to do with it. **One REVISIT per affected work-context.** When in doubt, split — the cost of two REVISIT lines is lower than the cost of a missed forward-ref. The same applies when the trigger crosses ownership (e.g. mixing DNS records into a `REVISIT(traefik port)` — DNS records get filed under `apps/dns`, not traefik).
- **"Today" / "Current" ambiguity in QUESTIONs and RISKs.** When stating current behavior, name the *codebase* (`legacy main-branch K2Secret lives in caller's chart`) rather than using a bare temporal word (`Current K2Secret lives in caller's chart`). On a topic that's mid-migration, "today" / "current" could mean either side of the cut. A fresh agent reading the entry doesn't know which.
