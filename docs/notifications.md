# Notification sounds

Open `:settings` and select **notify sound**. Use Left/Right to cycle through
25 built-in sounds, plus `default` and `bell`. Each change previews and saves
the choice, even when notifications are off. Choosing a sound clears any custom
notification command. Short terminals scroll to keep the selected setting visible.

Commands:

- `:notify sounds` lists the available names.
- `:notify sound chime` selects, saves, and previews a sound.
- `:notify test` previews the current signal without changing notification mode.
- `:notify off|question|done` controls when signals occur. Notifications are off
  by default. `question` signals questions from the agent; `done` also signals
  successful or failed turns. Your own Ctrl+C does not signal. This applies to
  background sessions too.

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

`:notify command <cmd>` overrides the selected sound. Commands run via `sh -c`
in the background (five-second timeout), with `MENO_NOTIFY_EVENT` (`question`,
`done`, or `error`), `MENO_NOTIFY_SESSION`, and `MENO_NOTIFY_TITLE` in the environment.
`:notify command bell` forces the terminal bell. `:notify command` without an
argument clears the override and restores the selected sound. An old
`notify_question: on` setting migrates to `notify: question`.
