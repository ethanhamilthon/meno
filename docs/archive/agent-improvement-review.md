# meno: Session Review and Improvement Backlog

Date: 2026-10-17
Status: historical analysis and proposals. The subsequent design and delivery
specification is in [the implementation plan](plan.md); it supersedes this
review's provisional scope and ordering.

The ongoing design interview is recorded in
[design decisions and open branches](design-decisions.md).

## Scope and evidence

Reviewed 20 substantive sessions from October 16–17 in
~/.meno/meno.db and compared their messages, tool calls, and recorded
errors with the current prompt, JS VM, MCP bindings, and compaction code.

The sample contains 52 records with a nonempty error field. Nineteen concern
redeclared JS identifiers: approximately 37% of recorded errors. This is not a
complete failure rate: shell failures can appear inside normal tool output,
and persisted history is not a complete benchmark of model quality.

Main conclusion: predictable tools, isolated development sessions, and explicit
completion criteria offer more immediate value than simply longer prompts.

## 1. Tool reliability

### Persistent JavaScript variable collisions

Repeated errors include identifier redeclarations, assignments to constants,
and references to variables whose initialization failed. They occur across
coding, Pencil, and Notion sessions, not just with one model.

Recommendations:
- Document a safe default: temporary variables belong in a local block.
- Keep intentionally persistent state in an explicit state object.
- Consider a per-call local-scope execution mode without removing explicit
  persistence; define compatibility and return-value behavior before changing it.
- Explain persistent-scope recovery in relevant error messages.
- Add regression scenarios for repeated calls and failed initialization.

Example safe temporary scope:

~~~js
{
  const result = sh("git status --short");
  console.log(result);
}
~~~

Relevant code: internal/jsvm/vm.go and internal/prompt/prompt.go.

### MCP result assumptions

Pencil sessions repeatedly treated Markdown as objects, parsed non-JSON text,
confused export directories with image paths, and used browser tools on canvas
nodes. The October 17, 13:22 TUI-to-Pencil session illustrates this overhead.

Recommendations:
- Provide explicit text, structured-data, and artifact fields where supported.
- Preserve text responses as text; do not imply every response is JSON.
- Read unfamiliar tool schemas and inspect a small response before building
  a large script around assumed behavior.
- Put tested Pencil recipes in a skill: find, clone, export, and inspect frames.
- Improve existing adapters before adding more external integrations.

Relevant code: internal/jsvm/mcp.go and internal/prompt/prompt.go.

### Editing and long-running commands

Add focused capabilities through JS rather than necessarily adding more
provider-visible top-level tools:
- Range-based file reads and structured code search.
- Patch application with an expected file version or content hash.
- Useful diagnostics for failed or ambiguous edits.
- Background process start, status, logs, and cancellation.
- TUI screenshots and visual comparisons, including resize scenarios.

Preserve refusal of ambiguous edits. Do not make fs.edit silently replace
multiple matches merely to avoid errors.

## 2. Parallel development isolation

In the October 17, 13:04 attachments/steering/notifications session, changes
were mixed with pre-existing work. The agent repeatedly deferred commits and
reported a temporary test file appearing and disappearing during validation.
This indicates shared-worktree coordination risks, not proof of the origin
of every individual edit failure.

Recommendation: give each code-writing session a separate Git worktree.
Read-only analysis, design, and Notion work do not necessarily need one.

Until isolation exists, use one code-writing session per working tree; other
sessions should remain read-only or work on separate artifacts. Worktrees
isolate files, not automatically shared databases, ports, or external services.

## 3. Context construction and persistence

### Separate context layers

- Permanent rules: architecture, style, validation, and Git policy.
- Current task: goal, scope, exclusions, and acceptance criteria.
- Working state: decisions, completed work, remaining work, and blockers.
- Sources: files, session messages, frame IDs, and validation results.

AGENTS.md, repo maps, reusable skills, and written handoff plans already work
well. Extend them without putting every detail into the system prompt.
Remember decisions and their reasons, not merely which files were read.

### Structured compaction

Current internal/agent/compact.go requests a concise summary preserving
choices, file paths, and results, then keeps a tail of four records while
protecting replay groups. That tail is not necessarily four user exchanges.

Proposed continuation summary:
- Goal and acceptance criteria.
- Active decisions and explicitly superseded decisions.
- Completed and remaining work.
- Constraints and explicit exclusions.
- Changed files, worktree, and Git state.
- Validation performed, results, and unverified behavior.
- Selected visual artifacts and their stable identifiers.
- Next concrete action and blockers.

Treat the summary as a compact index, not authoritative replacement evidence.
Allow retrieval of original messages when a decision needs verification.

### Session retrieval

Provide read-only search and bounded reads, conceptually:

~~~text
sessions.search(query, project, dateRange)
sessions.read(id, range)
~~~

Return excerpts with session IDs and record positions. Exclude thinking and
large tool outputs by default. Use retrieval to recover decisions, resume
unfinished work, and hand tasks between models without rereading everything.

### Design provenance

Welcome-screen sessions required restoring earlier variants after the agent
changed or removed them. Preserve alternatives unless removal is requested.

- Give variants stable names and IDs.
- Record the selected frame and reference screenshot in task state.
- Freeze the chosen visual specification before implementation.
- Specify animation timing, movement, resize behavior, and static appearance
  separately; a screenshot alone does not define animation.

## 4. Agent behavior

### Continue an approved plan

In the attachments/steering/notifications session, the user repeatedly had to
say “continue” after approving the overall plan.

Continue until the agreed scope is complete. Ask only for genuine blockers,
material scope changes, or decisions requiring separate user input. A commit
ownership question should not automatically block unrelated implementation.

### Check requirements before declaring completion

The attachments implementation was described as ready while the same report
noted that normal-mode d/x/c operations could split image tokens, despite the
planned atomic behavior.

Track requirement → implementation → test → status. Report partial completion
when agreed requirements remain unresolved. Explicitly negotiate deferrals.

### Investigate implementation before explaining it

In the October 16, 19:47 await session, the agent initially reasoned from general
JavaScript assumptions. The user had to point out that it had not read the code.

For repository-specific questions, inspect the implementation before making
confident claims. Distinguish observed behavior from assumptions.

### Use experiments to distinguish hypotheses

In the October 17, 14:37 Gemini cache session, log analysis was followed by a
controlled repeated-request experiment only after several user follow-ups.
The later experiment reproduced the uncached remainder outside meno.

Investigation protocol:
1. State observations.
2. List competing hypotheses.
3. Run a minimal check that distinguishes them, when feasible.
4. Separate proven findings, likely explanations, and unknowns.

Do not confuse a plausible arithmetic explanation with an established cause.

## 5. Prompting recommendations

Existing strengths: concrete feedback, file/frame references, requests for
edge-case validation, reusable skills, and challenges to unsupported claims.
Longer prompts are not inherently better. Add scope and a stopping condition.

### Project assessment

“Assess meno for daily coding work. Do not change code. Rank three improvements
by reliability and speed, support them with code or recent sessions, and
recommend one next action.”

### Implementation

“Add 25 notification sounds. Provide selection and preview in settings, persist
the selection, and preserve custom notification commands. Finish the agreed
scope without intermediate approval requests. Run tests and a build. Do not
commit.”

### Investigation

“Find the cause of low cache hit. Do not change code or settings. Distinguish
meno, proxy, and upstream hypotheses. If logs are inconclusive, run a minimal
comparative experiment. Report proven, likely, and unknown findings separately.”

### Design

“Create five variants on new frames. Do not modify or delete existing variants.
Record their names and IDs. Implement only the variant selected afterward.
Respect terminal-grid constraints.”

### Selective grilling

“Ask only questions whose answers materially change architecture or UX. Make
minor decisions yourself and briefly list assumptions.”

Use a small task contract: desired result, acceptance criteria, autonomy and
stopping rules, and commit policy. Avoid answering long lists of minor choices
that the agent could reasonably decide itself.

## 6. Suggested priorities

These are recommendations, not an approved delivery order.

- P0: safe JS execution patterns and regression tests.
- P0: isolated worktrees for parallel code-writing sessions.
- P1: persistent task state and acceptance-aware completion/compaction.
- P1: session search and bounded history retrieval.
- P1: range reads, code search, and version-aware patches.
- P1: background processes and predictable MCP result handling.
- P2: TUI visual snapshots and comparison workflows.

Immediate workflow change: one code-writing session per worktree, with explicit
acceptance criteria and commit policy.

## 7. Future capabilities: TODO, tasks, and agents

Status: idea capture only. Design is intentionally unresolved. The examples
below are not implemented APIs, approved contracts, or an implementation plan.
Revisit semantics, persistence, UI, and permissions before implementation.

### TODO: agent work checklist

Capture the idea of an agent-accessible checklist, for example todo.list() and
todo.done(n). Other operations, item identifiers, ordering, and lifecycle remain
undefined. Do not infer an API beyond these illustrative examples.

Open questions: session-local or shared; persistence and compaction; whether
items represent acceptance criteria or work steps; user visibility and editing;
how to handle stale items and distinguish “done” from verified completion.

### Tasks: background work beyond an agent turn

Tasks are background jobs that can survive the end of an agent turn. Typical
uses include rendering, development servers, and other long-running processes.
They are distinct from TODO checklist items.

Open questions: API names; start/status/logs/cancel behavior; ownership by
session or worktree; output retention; completion events; cleanup; ports and
resources; behavior on session deletion and application exit. Surviving a turn
does not yet imply surviving an application restart or machine reboot.

### Agents: subagents and delegated work

Capture the idea of spawning subagents for delegated work. No orchestration
model, API, or UI is selected yet.

Open questions: context inheritance and budget; model selection; tool access;
read-only versus code-writing roles; worktree isolation; concurrency; progress
and results; cancellation; validation; costs; and parent-agent responsibility
for checking and integrating delegated output.

Keep TODO state, background process lifecycle, and subagent orchestration
conceptually distinct. Their relationships are part of the future design.

## 8. Additional fixes and first-release sequencing

See [release blockers and input requirements](../release-blockers.md) for
cache, highlighting, TUI performance, file-picker, and atomic-object work
to complete before first-release preparation.
