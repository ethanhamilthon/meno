# Notification sounds

Open `:settings` and the **Notify** tab:

| Setting | Values |
| --- | --- |
| notify | `off` (default), `question`, `done` — when meno signals that you are needed |
| notify sound | `default`, `bell`, or one of the built-in sounds; changing it previews the sound |
| notify volume | `20`, `40`, `60`, `80`, `100` (default `100`); changing it previews the sound |
| notify command | an optional shell command that replaces the built-in sound; `bell` forces the terminal bell |

Volume scales the built-in samples. It does not apply when **notify command** is
set: a custom command makes its own sound. Clearing **notify sound** is not
needed: choosing a sound clears a custom command.

Notifications are global: a background session that needs you signals too. Your
own Ctrl+C does not signal. `question` signals questions from the agent;
`done` also signals the end of a turn, with success or error.

## Built-in sounds

chime, bloom, droplet, spark, soft-bell, marimba, orbit, ripple, ping,
double-ping, triple-ping, doorbell, harp, music-box, sunrise, moonlight,
pixel, arcade, sonar, woodblock, knock, triangle, success, attention, gentle.

These are original synthesized cues, generated locally without downloads or
system sound assets. Playback uses `afplay` on macOS, or the first available of
`paplay`, `aplay`, and `ffplay` elsewhere. If no player is available or playback
fails, meno falls back to the terminal bell. Temporary WAV files are removed after
playback. Playback runs in the background with a five-second timeout.

`default` preserves the old behavior: macOS Glass, terminal bell elsewhere.
`bell` always uses the terminal bell. Terminal settings may mute bell playback.
The selected name is persisted as `notify_sound` in settings; old settings files
keep the platform default. Unknown names reset to `default` when loaded.

## Custom commands

A custom **notify command** runs via `sh -c` in the background (five-second
timeout), with `MENO_NOTIFY_EVENT` (`question`, `done`, or `error`),
`MENO_NOTIFY_SESSION`, and `MENO_NOTIFY_TITLE` in the environment. The volume
setting is ignored. Leave the field empty to return to the selected sound.
An old `notify_question: on` setting migrates to `notify: question`.
