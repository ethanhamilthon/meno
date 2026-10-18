# Atomic Objects, Path Completion, and JS Rendering

Part of [the implementation specification](../plan.md). Review draft.

## Message representation

Represent message content as text plus explicit typed object segments with stable
identity/metadata. Serialize for providers separately from rendering. Do not rely
on editing the characters of a display token to preserve object integrity.

Objects: images (stored attachment ID), skills (name and selected file path), files
(fixed absolute path), folders (fixed absolute path). Persist segments so restored
messages, queue recall, edits, undo and fork retain identity. Legacy plain paths
remain plain text; no automatic conversion merely because a file exists.

    file:   <file-link>ABSOLUTE_PATH</file-link>
    folder: <folder-link>ABSOLUTE_PATH</folder-link>
    skill:  <skill>name</skill>

Escape &, <, > in XML-style text and decode reversibly. Expose selected skill path
through trusted selection metadata/catalog mapping if names collide. Explicit skill
selection pins the chosen file, not a new same-name resolution after cwd changes.

No file/folder content injection, snapshots, existence tracking, missing-file badges,
rename following or target substitution. :cd does not change existing references.
Images remain actual stored attachments, not mere file-link substitutions.

## @ completion

Open in insert mode at input start or after whitespace, not within email/words.
Until acceptance the @ query is text. Esc closes the list and preserves that text.
Search relative to current cwd, supporting ../, ~/ and absolute paths explicitly.

Do not apply .gitignore or exclude .git, dotfiles, dependencies or build output.
Use asynchronous bounded search with stale-query cancellation; never scan an
unbounded tree on the UI thread. Return deterministic fuzzy path matches with
exact/basename priority. Show relative path context sufficient to distinguish names.

Show symlink entries but do not automatically recurse into symlink directories.
Explicit path/Tab navigation may enter them. Preserve the selected symlink path,
not realpath target. Protect traversal from cycles when explicit navigation occurs.

Up/Down selects. Enter creates one file/folder object. Tab enters/completes a folder
path for further search; Tab on a file accepts it. Folder entry and folder reference
creation are separate actions. Reopen with @ for another object; no multi-select.
Handle spaces/Unicode in filesystem names without splitting accepted references.

Skill autocomplete uses explicit Enter/Tab acceptance too. Typed /name alone remains
text; <skill> means read the selected skill file, not automatically inject its body.

## Editing invariants

Treat each object as one logical unit for cursor movement, word/character operators,
selection, replacement, mouse interactions, cut/copy/paste, and all Vim modes.
Selections intersecting any rendered part expand to the whole object. Backspace,
Delete, x, d/c/y, repeated commands, visual blocks, and undo/redo cannot split it.
Replacing an object removes it whole; its internal label is not editable text.

Display short highlighted @name or /name labels and [Image #N]. Labels may wrap
across visual lines but remain one logical unit. Map mouse positions on any wrapped
piece to that object. Preserve atomic selection across resize/scroll.

Apply to composer, recalled queue, drafts, edited messages, and previously sent
user messages, including restored history. Application selection/copy respects
objects; native terminal selection cannot be fully controlled and is documented.

Old images require real attachment metadata. Highlight legacy skills only when
stored evidence identifies them; do not reinterpret history using today's catalog.
Do not rewrite old text while rendering or fabricate unavailable object metadata.

## Clipboard

Internal registers and undo retain object data/references, including images.
External text copy emits full file/folder tags, explicit <skill> tags, and image
text labels; image bytes are not promised in external text clipboard transfer.

On paste, restore only well-formed full tags with absolute file/folder paths.
Restore skill tags only for available skills with unambiguous selection. Ignore
object syntax inside code spans/fences. Invalid, relative, malformed or unclosed
tags remain text. Untagged paths are plain text. Never attach bytes solely because
pasted text resembles [Image #1]. Clipboard-image attachment remains an explicit
supported input path distinct from textual image-token restoration.

## JS tool source highlighting and folding

Only syntax-highlight fully received JS tool-call source, before/during execution.
Streaming fragments remain readable unhighlighted text. Do not wait for the process
to exit. Use a JS-aware tokenizer/parser with fixtures for modern syntax, regexes,
comments, escapes, Unicode, templates and incomplete/error source fallback.

Strings remain string text, even when they contain HTML, SQL, Markdown or code.
No embedded-language highlighting. js.strings.hide has off (default) and lines N.
Off shows all. N>=0 shows first N content lines then a distinct muted ellipsis when
content was omitted; N=0 shows only the marker for nonempty content. Preserve
quotes/delimiters and surrounding JS; permit expand/collapse without source mutation.
Count logical literal content lines for stable folding independent of terminal width.

Keep template interpolation expressions visible as JS; fold only literal segments.
If segmentation is unsafe, render the whole literal. Never execute expressions for
rendering. Copy/export/execution/model replay use full original source, not folded
text. This setting does not hide assistant responses or tool output. Keep current
assistant Markdown behavior; add no multi-language highlighting to its text blocks.

## Acceptance

Property-test that arbitrary edit/undo/motion sequences never leave partial objects.
Test all modes, multi-object/text selection, wrapped labels, wide graphemes, mouse,
queue recall, paste/code examples, legacy sessions, fork, and narrow/resize layouts.
Test JS token styles and fold mappings against editor-quality syntax fixtures.
