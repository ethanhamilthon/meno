# Continuous Design Interview: Additional Decisions

Status: decisions collected during the final continuous interview round.
This file supplements earlier interview notes; explicit supersessions win.

## Context and retrieval

- Automatic compaction triggers before an LLM request when estimated input plus
  response reserve reaches 80% of a known context window. Defaults: at least
  eight tail messages (expanded for protocol groups), summary budget 4K tokens.
  All three are configurable. Use the current model for generated summaries.
- Unknown context windows require an explicit model setting; do not invent a
  limit. Avoid repeated compaction loops when no progress is possible; fail
  clearly if the request still cannot fit. Preserve prior context on failure.
- Search all local user-facing sessions, not only the current project. This
  supersedes the earlier default project restriction. Workdir is an optional
  filter, not an access boundary.
- sessions.search(query, {workdir, after, before, limit, cursor}) uses local
  full-text search over titles/messages, no embeddings requirement.
- Results include session/message IDs, title, workdir, lastMessageTime,
  provider/model, excerpts, and pagination. Default 20 matches.
- sessions.read(id, {from, limit, includeToolOutput, includeThinking}) defaults
  to 50 records within 64 KiB, excluding thinking/large tool outputs.
- sessions.list({workdir, after, before, limit, cursor}) lists user sessions,
  newest lastMessageTime first, default 20 and maximum 100 per page. No query
  required. All retrieval operations are read-only and make no model calls.
- Exclude child sessions from general search/list; the parent may read a known
  child-session ID. Metadata fields do not contain credentials.
- Q90 synchronous versus asynchronous JS received no answer; still unresolved.

## Subagents: supersessions and API

- Remove read-only/full modes and their technical allowlists. The main agent
  communicates restrictions in the child task prompt. These are instructions,
  not a security sandbox. Retain enforced prohibitions on child askUser,
  background tasks, and nested subagent spawning.
- Final result envelope: {id, status, result, error, usage}, with text result.
  Do not add changedFiles or checks fields. Verification may be described in
  the result text; the parent remains responsible for evaluating it.
- agents.spawn({task, cwd, provider, model, effort}) returns {id} immediately.
- Parent API: list(), status(id), send(id, text), wait(id, {timeout}), result(id),
  cancel(id). list covers its children. result is repeatable. Wait timeout does
  not cancel the child.
- Child API: agents.report(text) for progress and agents.block(reason) to wait
  for the parent. Children cannot control sibling agents.
- Sending to a completed/failed child resumes its session under the current
  root request. No automatic revival of a request cancelled through :stop.

## Interactive questions and state limits

- New askUser accepts {questions:[{id,text,type,options,recommendation}],timeout}.
  Types: single, multi, text. Options use stable id/label pairs. Return
  {answers:[{id,selected:[optionId],text}]}. Drop the old string overload.
- Maximum 20 questions and 20 options per question. Validate before rendering.
  Every question is required and supports custom text. Waiting pauses JS timeout.
- Cancel/timeout throws a typed exception, sends no partial answers, and discards
  unfinished drafts. After restart, mark an unfinished questionnaire interrupted;
  do not resume automatically. Persist submitted questions and answers.
- State limits: 1 MiB JSON per value, 10 MiB per session, configurable. Reject
  excess before writing. Do not inject values into prompt automatically; give
  a short reminder after compaction/restart. Store large artifacts on disk.

## TODO operations and honest completion

- todo.list/add/update/remove/archive; todo.done(id) convenience operation.
  Separate todo.criteria.list/add/update/remove. Persist mutations immediately
  with history. Exact argument schemas are specified in the working spec.
- Validate criteria before final reporting. Show Unverified/Blocked for incomplete
  acceptance, but allow partial final reports; do not force an infinite model loop.
  Deleted/deferred requirements remain auditable and cannot hide unfinished scope.

## Filesystem and MCP tools

- fs.read(path,{startLine,endLine}) uses inclusive one-based line bounds; full
  no-options read keeps its string result. Add structured fs.search(query,
  {root,glob,regex,caseSensitive,limit,cursor}); literal search by default,
  ignore-aware with explicit includeIgnored. This differs from unfiltered @ search.
- fs.inspect returns text and whole-file sha256. fs.patch(path,edits,
  {expectedHash}) validates unique, nonoverlapping replacements against original
  content and rejects hash conflicts before writing. Atomic per file, not a
  multi-file transaction. Preserve strict fs.edit and improve match diagnostics.
- MCP tool calls return {text,structuredContent,content,artifacts,isError}.
  Never parse arbitrary text as JSON automatically. structuredContent only comes
  from explicit server data. Preserve typed MCP content and real artifacts.
- Transport/protocol failures throw; MCP tool isError returns in the envelope.
  Large results need bounded retrieval with a stable reference, not silent loss.

## MCP artifacts and terminal graphics

- mcpResults.info(id) and read(id,{offset,limit}) retrieve locally stored full
  results without re-invoking external tools. Store binary blocks as artifacts,
  not huge prompt base64. Retention follows the owning session history.
- images.protocol supports auto, off, kitty, sixel, iterm. Auto uses confirmed
  capabilities, preferring Kitty, then iTerm, then Sixel if several work.
  Use tmux passthrough only when supported; otherwise text fallback.
- Safely hide a partially visible image's graphics while retaining its caption
  if a backend cannot clip it correctly. Never paint over unrelated UI regions.

## Performance and JS rendering

- Profile reproducible long-history, streaming, image, resize, and scrolling
  workloads. On a documented baseline machine target p95 input/scroll response
  <=50 ms and standard frame work <=16.7 ms. CI uses deterministic workloads
  and relative regression thresholds, not fragile runner-wide absolute timings.
- Supersede the proposed multi-language syntax highlighting: highlight only
  completed JS tool-call source. Completion means full code received, before or
  during execution, not waiting for the process itself to exit.
- Strings inside JS remain string text, including embedded HTML/code; do not
  apply embedded-language highlighting. Provide js.strings.hide: off (default)
  or lines N. N=0 shows only an ellipsis for nonempty string content; N>0 shows
  the first N content lines plus a distinct muted ellipsis when truncated.
- Retain delimiters and surrounding JS. Expand/collapse hidden content via
  interaction. Preserve actual source, execution, copying, and model context.
- Fold only using valid string/template syntax boundaries after source completion.
  Keep template interpolation expressions visible as JS; if safe segmentation
  is not possible, show the whole literal rather than hiding executable code.
- This setting does not fold assistant responses or tool output. Ordinary
  assistant Markdown remains unchanged. The intermediate Q105/Q106 proposals
  about hiding assistant text were corrected by Q107/Q108 and do not apply.

## Diagnostics, release, and runtime

- Cache work: separate request/cumulative metrics; correct read/write usage;
  deterministic prefix/schema serialization; controlled meno versus direct API
  comparison. Default debug logs contain metadata/hashes, not payloads/secrets.
  Full payload logging requires explicit debug opt-in. No promised cache-hit %.
- Add test-only visual snapshots and regressions; do not add an agent ui.snapshot
  API. Validate real terminal graphics separately from tcell buffer captures.
- Design provenance uses state/plans and source IDs, not a new artifact manager.
  Preserve variants, record selections before implementation, document animation
  separately, and add tested Pencil recipes in a skill rather than core code.
- First release: GitHub MIT, macOS and Linux arm64/amd64 binaries, checksums,
  source build and go install instructions, changelog, CI. Windows, Homebrew,
  Docker, and auto-update are not required release scope.
- Serialize agent continuations per session. Deliver old task events at safe
  boundaries or after an open form; preserve original request ownership.
  Never revive cancelled requests or run two parent loops concurrently.
- No mandatory delegation cost/step budget. Show child usage and known costs,
  aggregate without double counting, and mark unknown prices explicitly.
- Crash recovery retains committed data and marks unfinished work interrupted;
  no automatic command/model retries on restart. Best-effort process cleanup
  verifies identity, never kills using a stale PID alone. SIGKILL cleanup cannot
  be guaranteed; document the limitation.
- Deleting a session stops its work and removes owned histories/state/TODO/logs;
  shared artifacts are garbage-collected only when unreferenced.
- JS runtime is fresh per invocation. Only explicit state and managed Go-side
  resources survive. globalThis/builtin/env mutations do not survive. A local
  env copy affects shell/tasks launched in that call.
- Fork/reset decisions are in [session forking](design-fork.md), superseding
  the intermediate Q116 reset description.
- Link paste uses strict full tags, absolute paths, XML entity escaping, and
  code-span/fence exclusion. Invalid or relative tags remain text. Restore
  <skill> tags only for available skills.
- session.compact validates and schedules one operation per tool batch, returning
  {scheduled:true}. Apply after all tool results, even if later JS fails; report
  actual retained count in an event before the next model request. Preserve
  source history. Repeated scheduling in a batch throws.

## Final steering and presentation corrections

- Completed children have no user-facing history viewer or special completion
  manager. Keep history internally for the main agent; :agents is working-only.
- Selected skill objects pin a concrete skill file, including duplicate names.
- All input/completion/reporting uses one steering system, not a separate task
  continuation queue. This supersedes the earlier old-request continuation model.
- Event A arriving during active request B retains source A; newly launched work
  belongs to B. If idle, continuation uses source ownership. :stop reads the
  actual active root request and never auto-revives a cancelled root.
- Approved dependency-first sequence: baseline/storage; full history/versions/fork;
  runtime/state/session/file/MCP contracts; steering/ownership; forms/TODO/tasks/
  subagents; atomic input/JS display; graphics; final cache/performance/acceptance;
  MIT/GitHub release. Investigation/profiling starts at baseline, not only at end.
