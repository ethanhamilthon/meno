# Inline Terminal Images

Part of [the implementation specification](../plan.md). Review draft.

## User-visible behavior

Render actual images in timeline for sent attachments and successful view() calls.
Use immutable attachment-store bytes so reopened history does not depend on source
files still existing. Keep user-image versus view-image provenance visibly distinct.

Scale proportionally, never crop source content to fill a thumbnail. Maximum width
is the message width; maximum preview height is half the visible transcript height.
Avoid unnecessary upscaling. Stack multiple images in message order and include
number/dimensions in text captions. A supported partial viewport may crop the
placement, not alter the image's stored content.

Composer contains atomic [Image #N] tokens only. No graphical composer thumbnails,
modal viewer, zoom/pan or automatic external opening. Image visibility is independent
of JS/tool-output hiding. Current accepted PNG/JPEG preparation limits remain unless
another explicitly approved change requires adjustment.

## Protocol setting and capability selection

images.protocol: auto (default), off, kitty, sixel, iterm.
Implement Kitty Graphics Protocol, Sixel, and iTerm inline image backends. Test
Kitty/Ghostty explicitly; do not assume an environment name guarantees capability.

Auto uses bounded capability detection and chooses a confirmed backend, preferring
Kitty, then iTerm, then Sixel if several work. Route terminal replies through the
input owner so they do not appear as typed characters. Use tmux passthrough only
when supported; otherwise fallback. Manual mode enables the selected backend, but
runtime errors must fail safely rather than corrupt the TUI or hang startup.

Unsupported/off/error fallback: textual image block with number, dimensions, type
and availability status. Do not emit speculative unsupported graphics sequences.
If stored bytes are unavailable, keep the message/caption and report it gracefully.

## Rendering responsibilities

Keep layout/viewport image placements in internal/ui; application owns session and
terminal lifecycle wiring. Isolate terminal graphics encoding/transport behind a
backend boundary. Integrate with tcell's terminal ownership; no unsynchronized raw
writes from agent goroutines. Preserve cursor, alternate-screen and input behavior.

Reserve layout rows/cells even while decoding. Upload/encode asynchronously with
bounded cache memory and artifact-keyed identity, avoiding repeated transfer on
every token/frame. Delete stale placements on scroll, resize, modal opening,
session switching, and exit. Do not leave pictures over composer/help/dialogs.

Clip to transcript viewport when a backend supports it safely. If partial visibility
cannot be rendered correctly, hide graphics for that placement and retain caption
rather than drawing outside bounds. Restore when fully visible again. This fallback
is allowed for backend limitations, not a reason to skip scrolling tests.

## Validation

Use deterministic layout/backend-command tests and real-terminal scenarios. A
PNG of tcell's text buffer does not prove terminal graphics rendering works.

Matrix: supported terminals/protocols, auto/off/forced, unsupported terminal, tmux
supported/unsupported, narrow/tiny dimensions, partial viewport, rapid scroll,
resize, modal, session switch, restored history, mixed streaming, transparent PNG,
JPEG, multiple/repeated images, invalid bytes, missing store, and cache eviction.

Validate placement cleanup and terminal restoration on graceful exit/failure.
Record backend-specific limitations with reproduction and evidence. A protocol
encoder alone does not satisfy inline rendering acceptance.
