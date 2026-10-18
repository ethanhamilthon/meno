# Terminal graphics

Images in the timeline (sent attachments and `view()` results) are drawn with a terminal
graphics protocol when one is available. Setting `images`: `auto` (default), `off`,
`kitty`, `sixel`, `iterm` (`:settings`, or `images_protocol` in the settings JSON).

- Every image has a caption line (`▣ Image #1 · 640×480 · png · attachment`). Without
  graphics the caption is all there is.
- With graphics, rows are reserved under the caption: the picture is scaled
  proportionally, never cropped and never enlarged, at most the message width wide and at
  most half the visible timeline high. Several images stack in message order.
- A picture is drawn only when all of its rows are visible; partly scrolled out it is
  hidden (the caption stays) and returns when fully visible again. Scrolling, resizing,
  opening a dialog, switching sessions and quitting remove the pictures.
- Pictures are decoded and scaled in the background and cached (32 entries). A missing or
  unreadable stored file leaves the caption.
- Input never shows pictures: the composer holds `[Image #N]` tokens only.

## Choosing the protocol

`auto` looks at the environment (`KITTY_WINDOW_ID`, `TERM=xterm-kitty`, `TERM_PROGRAM` =
`ghostty`, `WezTerm`, `iTerm.app`, sixel-capable `TERM` values) and falls back to text.
meno does **not** ask the terminal what it supports, because the replies would arrive as
typed characters, so `auto` is a hint, not a confirmation. Inside tmux or screen `auto`
is off; force a protocol with the setting if your passthrough is configured. A forced
protocol the terminal does not understand prints garbage: switch back to `off`.

## Verification status

| | |
| --- | --- |
| Encoders (Kitty chunking and headers, iTerm OSC 1337, Sixel structure) | unit-tested: `internal/termimg` |
| Layout, placement, caching, removal on scroll/dialog/session switch, fallback | tested against a fake terminal sink: `internal/app/images_test.go` |
| Real Kitty, Ghostty, iTerm2, WezTerm, foot terminals | **not run** by the authors of this change; the byte sequences follow each protocol's documentation |
| tmux passthrough | not supported |

Known limits: cell pixel size comes from the window size ioctl and falls back to 8×16;
Sixel uses a fixed 216-colour palette; iTerm and Sixel pictures are removed by repainting
the screen, which can flicker; partly visible pictures are hidden rather than cropped.
