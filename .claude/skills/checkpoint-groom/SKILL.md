---
name: checkpoint-groom
description: Groom a checkpoint note that has accumulated bloat. Drains the `## Inbox` section by filing tagged entries into proper sections, then drops irrelevant history, compresses superseded context, de-duplicates within each tag category, re-evaluates outstanding items against current code at HEAD, and promotes REVISIT entries whose triggering condition has become true. **Always dispatches to an Opus subagent** — grooming is judgment-heavy and benefits from a fresh context window. Use whenever the user says "groom checkpoint", "compact checkpoint", "consolidate notes", "tidy checkpoint", "checkpoint is getting long", when `## Inbox` has more than ~10 entries, or unprompted when a `.checkpoint/<topic>.md` exceeds ~400 lines or hasn't been groomed in many sessions. Companion to the `checkpoint-notes` skill.
---

# checkpoint-groom

Checkpoint files grow. They accumulate command logs from runs that no longer matter, references to files that have been deleted, rejected approaches that are no longer instructive, TODOs that quietly got done without anyone striking them off, and inbox entries from cheap-capture sessions that need to be filed into their proper sections.

This skill walks a `.checkpoint/<topic>.md` end to end, decides what to keep, compress, or drop, files inbox entries into their structured destinations, and writes back a smaller file that still preserves every piece of context worth preserving.

This is **not** the same as Claude Code's context compaction (`/compact`). That operation summarizes the conversation transcript. This operation tidies a checkpoint file on disk. Don't confuse them.

## Dispatch rule (mandatory)

**This skill ALWAYS runs in an Opus subagent.** The calling agent must spawn a sub-agent and delegate the work; do NOT perform the grooming inline.

Rationale:

- Grooming is judgment-heavy. It requires reading the full checkpoint, cross-referencing code at HEAD, and making conservative keep/drop/compress calls across many entries. Opus does this better than smaller models.
- The calling agent's context is typically already loaded with other work. Spawning a fresh subagent gives the grooming task an uncluttered context window.
- The subagent returns a concise summary; the calling agent relays it to the user without bloating its own transcript with the file's full contents.

### How to dispatch

When this skill fires, the calling agent's only job is to spawn the subagent and relay its result. Use the `Agent` tool with:

- `subagent_type: "general-purpose"`
- `model: "opus"`
- `description`: 3–5 word task summary, e.g. `"Groom v3-cdk-design checkpoint"`
- `prompt`: the template below, with placeholders filled in.

Then return the subagent's summary to the user verbatim (or near-verbatim — light reformatting OK; do not re-summarize).

### Subagent prompt template

```
You are grooming the checkpoint file at `<PROJECT_ROOT>/.checkpoint/<TOPIC>.md`.

Read the `checkpoint-groom` skill at `<PROJECT_ROOT>/.claude/skills/checkpoint-groom/SKILL.md` end-to-end and follow its procedure exactly. The skill defines:

- Pre-work: snapshot the file, capture current HEAD / branch / status, list the project tree relevant to the topic.
- Six operations in strict order: drain inbox → drop fully-irrelevant → compress superseded → de-duplicate → re-evaluate outstanding → promote met REVISITs.
- Post-work: bump header, re-order Action items, rewrite Resume, sanity-check section ordering, confirm `## Inbox` is empty at the end of the file.
- Hard rules and failure modes — read them; they are tighter than they look.

Also read the companion `checkpoint-notes` skill at `<PROJECT_ROOT>/.claude/skills/checkpoint-notes/SKILL.md` so you know the canonical file shape, the tag set, the caveman-ultra style, and the "Context only — not a plan of action" header rule. The grooming target should still match that skeleton after your changes.

Be conservative. When in doubt, KEEP. Cross-check against HEAD (the actual code) before trusting your memory of what got done.

Target topic: <TOPIC>
Project root: <ABSOLUTE_PROJECT_ROOT>

After grooming, report back in under 300 words:

1. Before/after line count (and a one-line "shrunk by N lines" summary).
2. What you dropped, what you compressed, what you promoted out of REVISIT, what you filed out of inbox — one bullet per category.
3. Anything you weren't sure about. If you left entries in inbox because you couldn't classify them, list them. If you noticed something missing that the file should probably capture but doesn't yet, flag it for the user (do NOT invent new entries during grooming).
4. The final HEAD / Branch / Updated values you wrote into the header.
```

Fill `<TOPIC>`, `<ABSOLUTE_PROJECT_ROOT>` (and `<PROJECT_ROOT>` everywhere it appears) before invoking. If you're not sure which topic file to target, run `ls .checkpoint/` first; if multiple files plausibly match, ask the user — do not guess.

### Spawning notes

- **Run the subagent in the foreground** (default). You need its result to report back to the user.
- **Do not parallelize multiple groomings.** One topic at a time. Grooming two related files concurrently produces sloppier merges than grooming each in focus. (This matches the "Touching multiple topic files in one pass" failure mode below.)
- **Do not retry on failure.** If the subagent returns saying something is off (e.g. the file is wedged, the user's intent is unclear), surface that to the user rather than relaunching.

## When to fire

- User says: "groom checkpoint", "compact checkpoint", "consolidate notes", "tidy checkpoint", "the checkpoint is getting long".
- Unprompted, when:
  - `## Inbox` has accumulated more than ~10 entries. (Inbox bloat is the most common trigger now that capture is cheap.)
  - A `.checkpoint/<topic>.md` exceeds ~400 lines.
  - The file has more than ~10 entries in `## Stale`.
  - A major milestone just landed (whole sub-project done; many TODOs at once).
  - You're about to handoff and realize the file is hard to skim.
- After a long session where many `DECISION:` / `SUPERSEDED:` entries piled up.

Do **not** fire on a checkpoint that's small and recently written **AND has an empty or near-empty `## Inbox`**. The bias is *toward* keeping things — only consolidate when the cost of bloat exceeds the cost of grooming. But inbox draining is cheap relative to the rest of the operations; if the inbox is the only thing that needs attention, doing the drain alone is a reasonable lightweight pass (still in a subagent).

---

# Subagent procedure

Everything below is what the subagent executes. The calling agent does not need to read it — it just needs to spawn the subagent per the dispatch rule above.

## Pre-work

1. **Read the entire target file.** No exceptions. You cannot consolidate what you haven't read.
2. **Snapshot before edits**: `cp .checkpoint/<topic>.md /tmp/<topic>.before.md`. If something goes wrong, you can diff.
3. **Capture current ground state**:
   - `git rev-parse HEAD` and `git branch --show-current` — for the new `HEAD:` / `Branch:` header.
   - `git status --short` — what's actually changed since the last update.
   - Run any verification command(s) the checkpoint relies on (e.g. `earthly +lint`) and note the result. Past results no longer matter; current result does.
4. **List the project tree relevant to the topic.** When deciding whether a `Files & paths` entry is still useful, you need to know if the path exists.

## The six operations

Perform these in order. Each is conservative — when in doubt, keep.

### 0. Drain the inbox

`## Inbox` is the **last section in the file** — the cheap tail-append zone where the `checkpoint-notes` skill drops new tagged entries without reasoning about destination. This operation moves each line to its proper section above so the rest of the file stays usable.

Procedure:

1. Read every line of `## Inbox` (at the end of the file). Each non-comment line begins with a tag (`TODO:`, `DECISION:`, `LEARNING:`, etc.).
2. For each entry, decide the destination section using this table. The tag determines the destination — you should not be re-deciding semantics, only routing.

   | Inbox tag | Destination section |
   | --- | --- |
   | `TODO:` / `DOING:` / `BLOCKED:` | `## Action items` |
   | `DECISION:` | `## Decisions` |
   | `CONSTRAINT:` | `## Constraints` |
   | `LEARNING:` | `## Learnings` |
   | `QUESTION:` / `RISK:` / `REVISIT(...):`  | `## Open` |
   | `SUPERSEDED:` | `## Stale` (wrap in `~~ ~~` strikethrough + add `← <replacement>`) |

3. Before appending an inbox entry to its destination, check whether the destination already contains a near-duplicate. If yes, merge (prefer the stronger / more concrete / more general wording per operation 3) rather than appending a redundant line.
4. **Verify each entry against the code before filing it.** Inbox entries are cheap captures — some may have been resolved between when they were written and now. Drop entries that are no longer true; don't file stale facts.
5. **Verify every `§/path/sha` reference in the entry against its source.** Open the design doc (typically `notes/<topic>.md`) and confirm that §6.7 actually exists and says what the entry attributes to it. The most common drift mode: a `§8.4 #11` reference that the design doc actually has as `§8.3 #11` (current-state pain points live in a different section than v3-changes-to-fix-them). If you find a wrong reference, correct it before filing — do not propagate the error.
6. **Enforce self-containment per the `checkpoint-notes` tag table.** A QUESTION must include current behavior, not just the open choice. A REVISIT must describe the action, not just the trigger. A RISK must state trigger + consequence. If the inbox entry is decontextualized, expand it from the design doc / code before filing — or, if you can't expand it confidently, leave it in inbox and flag to the user. Do not file a half-baked entry just to clear the inbox.
7. Once an entry is filed (or dropped), remove it from `## Inbox`.
8. After draining, `## Inbox` should contain only the leading comment block (the `<!-- ... -->` template guidance) and be otherwise empty. **Keep the section header at the end of the file** so the next capture has a tail-append target.

Edge cases:

- **Entries without a recognizable tag.** A capture-path agent might write a bare line. Don't drop it — promote it to whatever tag best fits and route accordingly. If it's truly unclassifiable, leave it in inbox and flag to the user.
- **Entries that contradict an existing entry.** Treat the inbox entry as the newer truth (it was captured more recently). Move the existing entry to `## Stale` as `~~SUPERSEDED:~~` with a pointer to the new entry, then file the inbox entry normally.
- **Entries that are obviously inbox-only thinking-out-loud** (e.g. "maybe we should X?"). These are not captures, they are noise. Drop with no replacement. The capture skill says to bias toward inclusion, so be lenient — only drop if it's clearly editorializing.
- **Inbox at the wrong position.** If you find `## Inbox` somewhere other than the end of the file (e.g. an old file from before the inbox-at-bottom convention), drain it normally, then **move the empty `## Inbox` header to the end** so the next capture lands in the right place.

### 1. Drop fully-irrelevant entries

Drop only when **both** conditions hold:
- The artifact the entry refers to no longer exists (file deleted, commit reverted, command no longer applicable), **and**
- The rationale embedded in the entry is no longer instructive (it's not a learning, it's not a constraint, it doesn't explain a still-true behavior).

Common drop targets:
- **Command logs** showing past successes or failures against code that has since been rewritten. Past lint errors against files that no longer exist → drop. Past `earthly +k8s-manifests` runs whose output is just confirmation that something worked at the time → drop.
- **Files & paths entries** for files that no longer exist in the tree.
- **Decisions about code that's been deleted entirely** (not refactored — *deleted*). E.g. "DECISION: extract BootstrapPolicy to bootstrap.ts" once bootstrap.ts has been deleted and the decision overturned. Move to STALE if any LEARNING came out of it; drop only if it's pure trivia.
- **TODOs that quietly got done.** Verify by reading current code, not by trusting your memory. Move to a brief "did X" line in `## Status` if the completion is consequential; just delete the TODO entry.

Never drop:
- A `CONSTRAINT:` that's still enforced.
- A `LEARNING:` that's generalizable beyond the specific situation.
- A `DECISION:` if the *rationale* still applies to anything in the codebase, even if the original target moved.

### 2. Compress superseded-but-still-relevant entries

Some `SUPERSEDED:` / `## Stale` entries are still load-bearing because they explain *why* the current state is the way it is. Don't drop them; compress them.

Compression patterns:
- "We considered X and rejected it because Y" → keep as a single SUPERSEDED line if Y is non-obvious or might come back. Drop if Y is now obvious from the code.
- A chain of decisions A → B → C where only C is current → collapse to one line: `DECISION: C (chose over A, B because <one-phrase reason>)`. The full chain doesn't need to live in `## Stale` if the final outcome captures the trade-off.
- Multiple successive SUPERSEDED entries about the same field → collapse to one entry showing the final state and a one-phrase note about the journey if it's instructive.

Keep these uncompressed:
- Anything where a future agent might re-derive the rejected approach without context. The SUPERSEDED entry is there to prevent re-treading.
- Anything tagged with a sharp LEARNING that the current code doesn't make obvious.

### 3. De-duplicate within categories

Walk each section and look for near-duplicate entries. Common offenders:
- TODOs that restate the same thing with slightly different wording (often a sign the file was updated multiple times without re-reading).
- Multiple DECISION lines that all point at the same choice, accumulated as the same point came up repeatedly.
- LEARNING entries that are special cases of a more general LEARNING already present.

Merge into the strongest, most-general version. Prefer the entry that:
- References a concrete file path or commit SHA.
- Uses the cleanest caveman phrasing.
- Has the broadest applicability (for LEARNINGS).

If two entries genuinely cover different things despite looking similar, leave both and tighten the wording so the distinction is clear.

### 4. Re-evaluate outstanding items

For every entry in `## Action items`, `## Open`, and any other future-facing section, ask: **is this still true?**

Specifically:

| Tag         | Re-evaluation question |
| ----------- | ---------------------- |
| `TODO:`     | Has it been done? (Check code.) Is it still wanted? (Has scope changed?) Is it still concrete? (If it's drifted into aspirational, either sharpen or drop.) |
| `DOING:`    | Is anything actually in flight, or did the session end without resuming? If stale, demote to TODO or drop. |
| `BLOCKED:`  | Is the blocker still real? Has the thing it waits on happened? If unblocked, promote to TODO. |
| `QUESTION:` | Still need user input? Did the user implicitly answer in a later turn? If answered, fold into a DECISION. |
| `RISK:`     | Still possible? Or has the surface area changed such that this risk is now moot? |

When in doubt about whether something got done, **check the code at HEAD**, not your memory of the session.

### 5. Promote met REVISIT conditions

For every `REVISIT(<condition>):` entry, check whether `<condition>` has become true since the entry was written. If yes:

- **Move the entry out of `## Open`** and into `## Action items` as a `TODO:`. The condition firing means the work is now actually actionable.
- Keep the original wording compact: `TODO: <what to do>. Was REVISIT(<condition>); condition met.`
- Don't silently drop the trigger phrase — losing the link from REVISIT → TODO costs a future agent a confused minute trying to figure out why a TODO appeared.

Common conditions worth proactively checking:
- "X app present" → does `apps/X/` exist now?
- "X CRDs imported" → does `apps/X/crds/crds.k8s.yaml` exist?
- "X helper exists" → grep for it.
- "X resolved upstream" → check the linked issue or release notes if possible.

If you can't tell whether the condition is met (e.g. it depends on user judgment), leave the REVISIT alone. False positives are worse than false negatives — promoting work the user didn't actually want to do is a worse failure than leaving a REVISIT entry parked.

## After the six operations

1. Update header: `Updated:` to today, `Branch:` and `HEAD:` to current values.
2. Re-order `## Action items` so the next concrete step is first.
3. Rewrite `## Resume` if anything in step 4 invalidated the previous next-step plan.
4. Sanity-check section ordering — the skeleton in `checkpoint-notes/SKILL.md` is canonical.
5. Confirm `## Inbox` is present (empty, with the leading comment block) at the **end of the file** so the next capture has a tail-append target.
6. Confirm the "Context only — not a plan of action" header block is present right after the H1 title. Re-add it verbatim if missing.
7. Final byte count: if the file is still longer than ~400 lines after consolidation, the topic is probably overloaded — flag it to the user and consider splitting into two topic files.

## Hard rules

- **Never delete a `CONSTRAINT:`** unless the constraint itself was explicitly retracted (in which case it goes to `## Stale` as a SUPERSEDED line).
- **Never delete a `LEARNING:`** without checking it's been internalized into AGENTS.md, code comments, or another durable surface. Otherwise the next agent will re-make the same mistake.
- **Never compress a `DECISION:` that has unresolved ambiguity**, e.g. one that says "X for now, may revisit". Convert those to `REVISIT(<condition>):` instead of trying to make the DECISION shorter.
- **Never invent new entries during consolidation.** This skill compresses what's there. It does not author new state. If you notice something missing, surface it to the user and let the next checkpoint-notes pass add it.
- **Never run consolidation in a fast pass.** If a file is large enough to groom, it deserves a careful read.
- **Never leave an entry orphaned in `## Inbox` after a full grooming.** Either file it, drop it (with justification), or — if you're genuinely unsure — leave it AND flag it to the user. Silent abandonment in inbox is the worst outcome because the next capture-path agent will append below it and the inbox grows unbounded.
- **Never reorganize during a capture-path update.** Inbox draining only happens in this skill (`checkpoint-groom`). If `checkpoint-notes` ever moves entries out of inbox inline, the cheap-capture contract is broken.
- **Never transcribe a `§/path/sha` reference without opening it.** Every section/file/line/commit reference must be verified against the source before filing. The cheapest tool for this is `rg '^## .*<keyword>' notes/<topic>.md` (or `git show <sha>`); use it. If a reference is wrong, fix it; if it can't be resolved, surface the entry to the user instead of filing with a guess.

## Failure modes

- **Over-aggressive dropping.** "I don't see why this would matter" is not justification — the absence of a SUPERSEDED entry can cause the next agent to re-attempt a rejected approach. When unsure, keep.
- **Trusting memory over code.** "I remember finishing that" is not evidence. Read the code.
- **Promoting REVISITs prematurely.** A condition that's "almost true" is still false. Wait.
- **Losing the trail between SUPERSEDED and DECISION.** When you collapse a chain of decisions, leave a one-phrase note about what was rejected so the trail still exists.
- **Touching multiple topic files in one pass.** Consolidate one topic at a time. The cognitive load of comparing across topics produces sloppier merges than focused per-topic work.
- **Filing inbox entries without verifying them against code.** Inbox is cheap capture — some entries may already be stale by grooming time. Don't propagate stale facts into the structured sections; verify each line against HEAD before filing.
- **Filing inbox entries without verifying their `§/path/sha` references.** A wrong section reference looks fine until the next agent follows it and finds nothing there. Common drift: §8.3 (current pain) vs §8.4 (v3 fix) — they have parallel numbering and entries get mis-routed during drain. Open the source doc and confirm before filing.
- **Filing decontextualized entries.** A QUESTION without current behavior, a REVISIT without an action, a RISK that's just a section reference — these survive grooming because the tag matches but they fail self-containment. The reader will grep for the tag, find the line, and have no idea what it means. Expand from source before filing or leave in inbox.
- **Inbox draining as semantic-rewriting.** This operation is *routing*, not authoring. If you find yourself rewriting an inbox entry significantly to make it fit a section, you're doing it wrong — file the entry as-is (tightened only for caveman style + the expansion needed for self-containment), or surface to the user. The "expansion from source" mentioned above is filling in the *referenced* context, not inventing semantics.
- **REVISITs bundling cross-cutting forward-refs slip through.** A `REVISIT(cert-manager port)` entry that mentions Authelia cookie domain (auth concern) and DNS records (dns concern) and K2Issuer email (genuine cert-manager concern) routes correctly into `## Open` but is still a defect: the auth-port and dns-port agents will never grep `REVISIT(cert-manager port)`. During grooming, scan every REVISIT and ask: "would the agent acting on this *specific* trigger expect to find each item here?" If any item belongs to a different trigger, **split into multiple REVISITs**. This is the one grooming operation that *does* author new entries (more lines, narrower triggers) — it's allowed because the original capture was malformed.

## Worked example

Before (snippets relevant to consolidation; `## Inbox` at file end as captured):

```
## Action items
TODO: port cert-manager from `main`
TODO: port cert-manager (CRDs + LE issuer + reflector + default cert)
DOING: investigating cert-manager
BLOCKED: cert-manager ← waiting on Argo CD merge of recent PR

## Open
REVISIT(argocd app present with CRDs): replace raw ApiObject ArgoApplication
REVISIT(traefik app + @k2/cilium web preset): flip argocd defaultDeny:false → defaultDeny:true

## Stale
~~SUPERSEDED: K2Volume.local() factory~~ ← dropped 2026-05-19 per node-ephemerality constraint.
~~SUPERSEDED: K2Volume.local() considered for Plex DB~~ ← dropped; use K2Volume.replicated.
~~SUPERSEDED: K2NfsVolume zone-attract hard-required~~ ← b568c9c made it soft (weight:100).

## Commands
$ earthly +lint                        # passed (run on 2026-05-18)
$ earthly +k8s-manifests               # passed (run on 2026-05-18)
$ earthly +lint                        # passed (run on 2026-05-19)

## Inbox
QUESTION: Helm strip-CRDs default — keep, or let chart own CRDs end-to-end?
LEARNING: cdk8s-plus auto-attaches volumes referenced by mounts.
LEARNING: cdk8s-plus auto-attaches volume mounts (no manual addVolume needed).
REVISIT(cert-manager port): K2Issuer email + AWS region hard-coded; move to cluster YAML.
TODO: §8.4 #4 validate app exports up front.
SUPERSEDED: bootstrap.ts BootstrapPolicy map — removed in 5ee314e.
```

After (inbox drained, sections groomed; `## Inbox` still at the end, now empty):

```
## Action items
TODO: port cert-manager from `main`. CRDs + LE issuer + reflector + default cert. K2Issuer email + AWS region: move to cluster YAML at port time (was REVISIT inbox entry).
TODO: §8.4 #4 validate app exports up front; print all missing exports at once.
TODO: replace raw ApiObject ArgoApplication w/ generated CRD binding. Was REVISIT(argocd app present with CRDs); condition met (apps/argocd/ exists, 83d3bcc).

## Learnings
LEARNING: cdk8s-plus auto-attaches volumes referenced by mounts → no manual addVolume / configureWorkload needed.

## Open
QUESTION: Helm strip-CRDs default — keep, or let chart own CRDs end-to-end?
REVISIT(traefik app + @k2/cilium web preset): flip argocd defaultDeny:false → defaultDeny:true.

## Stale
~~SUPERSEDED: bootstrap.ts BootstrapPolicy map~~ ← removed 5ee314e; bootstrap = provisioner CLI concern.
~~SUPERSEDED: K2Volume.local() factory + all local-volume usages~~ ← dropped 2026-05-19; nodes are ephemeral by design (CONSTRAINT). Use K2Volume.replicated for Plex DB / SQLite apps.
~~SUPERSEDED: K2NfsVolume zone-attract hard-required~~ ← b568c9c made it soft (weight:100).

## Commands
$ earthly +lint                        # green at HEAD
$ earthly +k8s-manifests               # green at HEAD

## Inbox
<!-- (empty; leading comment retained — next capture tail-appends here) -->
```

What happened (operation by operation):

- **Op 0 (drain inbox):**
  - `QUESTION: Helm strip-CRDs` → filed under `## Open`.
  - Two near-duplicate `LEARNING:` lines about cdk8s-plus auto-attach → merged into one in `## Learnings`.
  - `REVISIT(cert-manager port): K2Issuer ...` → merged into the existing cert-manager TODO as a port-time sub-task (the trigger condition matches the imminent work).
  - `TODO: §8.4 #4 validate app exports` → filed in `## Action items`.
  - `SUPERSEDED: bootstrap.ts ...` → moved to `## Stale` with strikethrough.
- **Op 1 (drop irrelevant):** `DOING: investigating cert-manager` dropped (session ended without resuming). `BLOCKED: cert-manager ← Argo CD merge` dropped (Argo CD now in tree).
- **Op 2 (compress superseded):** The two `K2Volume.local()` SUPERSEDED entries collapsed into one with a generalized rationale.
- **Op 3 (dedupe):** The two cert-manager TODOs merged into one with concrete sub-tasks inlined.
- **Op 4 (re-evaluate outstanding):** Repeated `earthly +lint` / `earthly +k8s-manifests` history compressed to current state.
- **Op 5 (promote met REVISITs):** `REVISIT(argocd app present with CRDs)` promoted to TODO (verified via `apps/argocd/` existing at HEAD, 83d3bcc).

The file shrank, the inbox is empty and ready for the next capture, and every piece of operationally-useful information survived.
