# Design Decisions: Inline Timeline Images

Status: confirmed interview decisions; implementation specification in progress.
See [the decision index](design-decisions.md).

## Scope and behavior

- Render actual images inline in the timeline for user-sent attachments and
  images successfully loaded through view(). Keep provenance distinct.
- Render from stored attachment bytes, including after reopening old sessions.
  Do not depend on the original source file still existing.
- Fit previews proportionally without cropping image content, no wider than
  the message and at most half the visible transcript height. Avoid unnecessary
  upscaling. Stack multiple images vertically in message order.
- Include a caption with the image number and dimensions.
- Only inline previews: no separate viewer modal, zoom, or pan functionality.
- In the composer, retain an atomic [Image #N] token without a graphical preview.
  Timeline rendering begins after submission or successful view().
- Image visibility does not depend on JS-code or tool-output visibility settings.

## Terminal graphics

- Support Kitty Graphics Protocol, Sixel, and the iTerm inline-image protocol.
  Kitty/Ghostty compatibility is an explicit validation target.
- Detect terminal capabilities and fall back to a text image block when graphics
  are unavailable. Do not emit unsupported graphics sequences speculatively.
- Text fallback includes image number, dimensions, and type.
- Exact auto-detection, manual protocol selection, tmux behavior, and protocol
  preference order remain to be specified. The earlier auto/off/kitty-only
  proposal was not accepted; support all three requested protocols.

## Implementation and validation constraints

- Coordinate graphics output with tcell rendering and terminal ownership.
  Do not mix unsynchronized raw output with ordinary screen drawing.
- Correctly clip images at viewport boundaries during scrolling; remove stale
  placements on resize, session switching, modal display, and application exit.
  Offscreen images must not draw over the composer or other interface regions.
- Cache decoding/transfers with bounded resources rather than retransmitting
  every visible image on every frame. Exact cache policy remains open.
- Validate protocol-specific behavior, partial visibility, long histories,
  streaming, narrow terminals, and restored sessions. Protocol support is not
  complete merely because an escape-sequence encoder exists.
