# JavaScript Runtime, State, Session, and File Tools

Part of [the implementation specification](../plan.md). Review draft.

## JavaScript contract

Create a clean goja runtime per invocation. Local declarations, globalThis changes,
prototype/builtin replacement, and env changes do not survive. Shared Go services
own durable state, task processes, child agents, MCP, and session data. Scope
isolation must not depend on agents remembering to wrap code in a block.

Host tools remain synchronous. Conservative default for final review: preserve
existing await-on-ready-value compatibility, without adding an event loop or an
asynchronous tool platform. Resolve/reject immediately settled results within the
invocation; pending async results fail explicitly and cannot outlive the runtime.
Audit current fixtures before replacing vm.run. Do not detect async syntax by
substring matching inside comments/strings. This default replaces no-answer silence,
not an earlier affirmative user choice; it is included in the final review.

Return the last top-level expression or an explicit top-level return. Compile
using syntax-aware transformation, not a regex rewrite. console.log is separate
printed output. Objects/arrays return readable JSON; primitives retain predictable
text. An unsupported result serialization fails explicitly, not as [object Object].
Printed output plus returned value share the current 64 KiB budget; truncation is
explicit and UTF-8 safe. Existing timeout/cancellation behavior remains bounded.
Waiting in askUser pauses the execution timeout, not the parent cancellation signal.

Env is a local copy. Overrides affect sh/tasks started within that invocation;
never write secrets into debug logs automatically. Pin active call cwd at start.

## State API

    state.set(key, value)          -> undefined
    state.get(key)                 -> deep copy | undefined
    state.has(key)                 -> boolean
    state.delete(key)              -> boolean (entry existed)
    state.listKeys()               -> string[]
    state.listValues()             -> copied value[]
    state.listEntries()            -> [key, copiedValue][]

Keys are nonempty strings, with literal dots/slashes. Sort enumeration by a defined
lexicographic string comparator, consistent across methods and restarts. Each list
uses a transactionally consistent snapshot. No state.clear and no property mutation
API. get of a stored null returns null, distinct from a missing key.

Strict JSON values only: null, boolean, string, finite number, arrays and plain
objects recursively. Reject undefined, holes, functions, symbols, BigInt, nonfinite
numbers, Date/Map/Set/custom objects, accessors, and cycles without invoking toJSON.
Report the failing value path. Do not silently omit or coerce unsupported data.

Validate and commit set immediately; on failure preserve the old entry and throw.
Successful writes survive a later exception/timeout. get/list return independent
copies, including nested objects. Default size limits: 1 MiB encoded JSON per value,
10 MiB current state per session, configurable. Reject overflow before writing.
Do not dump state into prompts: read explicitly; remind after resume/compaction.

## Session metadata

    session.rename(name) -> normalizedName
    session.models({provider?, refresh?}) -> {models, errors}

rename applies only to the current main session and persists/UI-updates immediately.
The child binding is unavailable. Trim outer whitespace; reject empty, multiline,
control-character, or over-six-word names. Titles need not be unique. Instruct the
main agent to name early, ideally four words, and rename on material task change
and first post-fork turn. Do not rename every substep.

models covers enabled configured providers and the UI's shared catalog/cache.
Return provider, model ID, known effort choices, context window, image support,
prices, stale/source metadata; unknown attributes are null, never invented.
Return per-provider errors with partial successes; refresh failures may retain
explicitly stale data. No credentials or claim that listed models were tested.

## Filesystem APIs

    fs.read(path, {startLine?, endLine?}) -> string
    fs.inspect(path, {startLine?, endLine?}) -> {text, sha256, startLine, endLine}
    fs.search(query, options) -> {matches, nextCursor, truncated}
    fs.patch(path, edits, {expectedHash}) -> {sha256, replacements}

Read/inspect use inclusive one-based line bounds. Missing bounds mean file start/end;
reject invalid/reversed bounds. No-options read retains its full-string behavior.
Inspect hashes the entire original file bytes, even for a requested excerpt.

Search options: root, glob, regex=false, caseSensitive=true, includeIgnored=false,
limit=100, cursor. Return path/line/column/text, deterministic order, bounded output,
and opaque pagination. Specify columns as one-based Unicode scalar positions.
Use explicit truncation and a continuation; do not silently lose matches.

Patch edits are {oldText,newText} replacements against original file contents.
Require expectedHash; every oldText must match exactly once; reject overlaps and
empty patterns. Check all before writing; atomic per-file replacement, preserve
permissions, no multi-file transaction promise. Coordinate app-owned writers and
recheck the version near commit; document limits against uncooperative external
writers, since filesystem rename is not a universal compare-and-swap primitive.

Keep fs.edit as strict unique replacement. Improve missing/ambiguous diagnostics
with count and bounded context/line references. Never silently replace all matches.
Maintain current filesystem access semantics, not a new implicit sandbox.

## Acceptance

Test repeated declarations, failure recovery, output budgets, return/expression
semantics, timeout, Unicode, strict JSON edge cases, restart persistence, DB failure,
state/fork versions, model partial failures, stale hashes, and ambiguous patches.
