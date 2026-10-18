# Design Decisions: Context, Skills, and Compaction

Status: confirmed interview decisions; implementation specification in progress.
See [the decision index](design-decisions.md).

## Skills

- An explicit <skill>name</skill> reference instructs the agent to read and follow
  that skill. The prompt catalog provides its path; do not automatically inject
  the whole file. Notify the agent when a skill changes so it can reread it.
- Removed or disabled skills remain references in historical messages, not
  activations of a different similarly named skill. Exact identity resolution
  for duplicate names and changed paths still needs specification.

## History and active context

- Persist complete original messages and tool results independently of active
  model context. Compaction must not delete or rewrite that original history.
- Timeline and bounded history retrieval retain access to originals through
  stable identifiers. Summary may cite those identifiers for verification.
- History deletion is an explicit separate operation, not part of compaction.

## Structured continuation summary

- Use Markdown sections, not a rigid JSON schema: goal and acceptance criteria;
  active and superseded decisions; completed and remaining work; constraints;
  changed files and observed Git state; checks and unverified behavior;
  artifact references/IDs; next action and blockers.
- Treat the summary as an index, not authoritative replacement evidence.
  TODO and acceptance-criteria storage remain their canonical source rather
  than being silently overwritten from summary text.

## Agent-triggered compaction

- Add session.compact(summary, saveLastNMessages). The agent supplies a ready
  Markdown string; this operation does not make an additional model request.
- saveLastNMessages counts messages of active history, not complete user tasks.
  It specifies a minimum tail, expanded as necessary to preserve tool-call and
  result groups. Zero is allowed, but the current compaction call/result group
  must still remain. Report the actual retained count after application.
- Apply only at a safe protocol boundary after persisting the tool result;
  exact scheduling and reporting API still require specification.
- Preserve previous context if compaction fails; retain full source history.
- Keep automatic compaction as a context-limit safety net, following the same
  summary sections and protocol-safe tail rules. Threshold, summary budget,
  default tail size, and model choice remain open.
