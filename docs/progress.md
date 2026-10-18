# Implementation progress

Status of the work described in [plan.md](archive/plan.md). "Done" means implemented and
covered by the tests named in the last column. Items marked **partial** or **not
done** say exactly what is missing; nothing here claims behavior that was not run.

| Area | Status | Evidence |
| --- | --- | --- |
| Additive migrations v2–v6, upgrade fixtures (run twice) | done | agent/history_test.go |
| Full history survives compaction, stable message IDs | done | agent/history_test.go |
| State store with version journal; state limits | done | agent/fork_test.go, jsvm/contract_test.go |
| :fork (safe boundary, no future summary, independent, legacy warning, missing cwd) | done | agent/fork_test.go |
| :new kept, :reset removed | done | term scenario 24 |
| Fresh JS runtime per call, return / last expression, strict results | done | jsvm/contract_test.go |
| `await`: settled values only, pending fails explicitly | done | jsvm/contract_test.go (no new event loop, as planned) |
| session.rename / models / compact; sessions.list / search / read | done | app/js_host_test.go, agent/retrieval_test.go, schedule_test.go |
| fs.read ranges, inspect, search, hash-guarded patch, edit diagnostics | done | jsvm/fs_tools_test.go |
| MCP typed results, artifact store, bounded read-back | done | mcp/store_test.go, jsvm/mcp_test.go |
| Steering queue, request ownership, request-scoped :stop, no auto-resume of cancelled work | done | agent/events_test.go, app/steer_test.go |
| askUser multi-question forms (navigation, Other, Cancel/timeout exceptions, mouse) | done | app/ask_test.go, term scenarios |
| TODO / criteria with evidence rules, archives, read-only collapsible view, fork versions | done | agent/todo_test.go |
| Background tasks: process groups, rotating logs, notify dedup, :tasks | done | proc/proc_test.go, app/tasks_test.go |
| Subagents: spawn, report/block/resume, prohibited bindings, :agents, usage accounting | done | app/children_test.go, children_more_test.go |
| Automatic compaction safety net | done | agent/autocompact_test.go |
| Cache: usage units, unknown ≠ 0, request fingerprints, :cache, opt-in payload log | done | llm/fingerprint_test.go, app/diagnostics_test.go |
| Benchmarks and per-turn layout cache | done | docs/performance.md |
| JS highlighting of completed source, string folding (`js strings` setting) | done | ui/jscode_test.go |
| @ picker (async, unfiltered, fuzzy, folders, symlinks) and atomic file/folder/skill objects in every editing mode | done | pathsearch tests, app/picker_test.go (random edit test) |
| Terminal graphics: Kitty / iTerm / Sixel encoders, layout, placement lifecycle, fallback | done against a fake terminal | termimg tests, app/images_test.go |
| MIT license, CI, 4-target release script with checksums, docs | done | scripts/release.sh run locally |

## Not done, partial, or not verified

- **Real terminals.** Kitty, Ghostty, iTerm2, WezTerm and foot were not available; the
  graphics output is verified against a fake sink only. `auto` uses environment hints;
  meno does not query the terminal. tmux passthrough is unsupported. Cell size falls back
  to 8×16 when the terminal reports no pixels.
- **Image tokens** (`[Image #N]`) are still text tokens: atomic in insert mode, not in
  every Vim operation. File, folder and skill objects are atomic everywhere. A label never
  wraps inside the composer (it is one cell group); in sent messages it wraps and stays
  highlighted as one unit.
- **Pencil skill.** `.meno/skills/pencil-design.md` gives the provenance rules; only the
  read-only `get_app_state` call was run. No recipe that edits a canvas was tested.
- **Session deletion lifecycle** (cancel and join managed work, delete children, results
  and logs) is not implemented because meno has no session-delete command yet; the result
  store has `DeleteSession` ready.
- **Crash cleanup by process identity** is not implemented: unfinished tasks and agents are
  marked `interrupted` at startup and never restarted, but orphan processes of a killed meno
  are not hunted down.
- **Cancellation generations** are simplified to request IDs plus the event state machine
  (`pending → delivered | acked | cancelled`); a cancelled request cannot auto-resume.
- **Performance targets** (p95 ≤ 50 ms input/scroll) were not measured on a real terminal.
- **Screenshot comparisons** beyond the existing visual fixtures were not added.
- **Oversized files.** Files I created or grew are split under 300 lines except `agent/loop.go` (336, was 379), `app/world.go`, `app/attach.go`, `app/modal_sessions.go` and a few older tests, which were already over the limit and received only small edits.

## Gates run for this state

`make fmt`, `make vet`, `make build`, `go test -race -count=1 ./...` (every package) and `make term` all pass. `make test` is a subset of the race run.
