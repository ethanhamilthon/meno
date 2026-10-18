# Design Decisions: Input Objects and Path Picker

Status: confirmed interview decisions; implementation specification in progress.
See [the decision index](design-decisions.md).

## Picker

- Open @ completion in insert mode at the beginning of input or after whitespace,
  not inside ordinary words or email addresses. Update results while typing.
- Until acceptance, the query is ordinary text. Esc closes completion and keeps
  that text. This Esc behavior is separate from the askUser questionnaire.
- Search from the session cwd; support explicit ../, ~/ and absolute paths.
- Do not apply .gitignore or exclude hidden files, dependency/build directories,
  or .git metadata. This supersedes earlier ignore-aware/exclusion proposals.
- Use fuzzy relative-path search, prioritizing exact and basename matches.
  Up/Down selects; Enter inserts the selected file or folder reference.
  Tab on a folder completes its path for navigation; Tab on a file accepts it.
  Insert one reference per selection; no multi-select requirement.

## Reference contract

- File selection serializes as <file-link>ABSOLUTE_PATH</file-link>.
  Folder selection uses <folder-link>ABSOLUTE_PATH</folder-link>.
- Show a short atomic @name label in composer and transcript; include enough
  path information to disambiguate matching names where needed.
- Highlight all supported atomic object types in previously sent user messages,
  not only in the active composer. Apply this to earlier messages in the current
  session and restored history after reopening a session. Application-controlled
  selection and copying must preserve object boundaries there as well.
- Render recognized objects in old messages without rewriting their stored text
  or automatically interpreting ordinary untagged paths as objects. Legacy
  images are recognized only with matching stored attachment data. Legacy
  skills are highlighted only when saved evidence identifies them as skills;
  do not infer them from the currently installed skill list.
- References contain paths only, never automatically embedded file contents.
- Do not track existence, renames, moves, or content changes. Do not copy or
  snapshot selected files. A cwd change does not alter an existing link.
- Missing-file badges and automatic reference maintenance are out of scope.
  Path escaping and malformed-tag parsing still need exact specification.

## Atomic editor behavior

- Images, skills, files, and folders each behave as one logical editor element.
- Left/Right move across the whole object. Backspace, Delete, and x remove it
  completely. Selections intersecting any part include the whole object.
- Apply atomic semantics to Vim motions/operators, d/c/y, replacement, repeated
  commands, undo/redo, and all message editors. Replacing an object with text
  removes the object; editing its label internally is not supported.
- Preserve objects in internal registers, yank/paste, undo, and redo.
- In the external text clipboard, serialize file/folder references as full tags
  with absolute paths; use explicit skill notation and text image labels.
  External text copying does not promise transfer of image bytes.
- Pasting valid file-link/folder-link tags restores atomic references. Ordinary
  untagged paths remain text. A typed image label must not acquire another
  attachment's image data. Exact skill clipboard notation remains open.

## Skill objects and layout

- Create skill objects only through confirmed autocomplete selection (Enter/Tab).
  A manually typed /name remains plain text, even if it matches a known skill.
- Show /name in UI; serialize an explicit skill object as <skill>name</skill>.
  Exact paste recognition and skill activation lifecycle still need specification.
- Allow object labels to wrap across visual lines while retaining one logical
  editor element. Selection, deletion, and movement remain atomic across wraps.
  Do not require truncation to keep a label on one line.
- Show symlink entries, but do not recurse into symlink directories during
  ordinary search. Explicit path navigation, including Tab, may enter them.
  Preserve the selected absolute symlink path instead of resolving its target.
