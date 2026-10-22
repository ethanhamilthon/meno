# meno update

Install the latest release for this platform. Takes no arguments.

Downloads and verifies the executable before requesting an idle-only daemon
shutdown. Active runs, queued messages and background tasks must finish or be
stopped first; there is no force flag. Shutdown failure prevents installation.
No available update leaves the daemon untouched; an absent daemon is not started.

Success leaves the daemon stopped. Close all meno windows and reopen meno to
start the new daemon. Existing sessions, settings and credentials are preserved.
Only the daemon selected by MENO_HOME is affected.

Older daemons without atomic idle shutdown require manual stopping first:
finish work, close all meno windows, run `meno daemon stop`, then retry.
An older installed updater does not gain automatic shutdown retroactively.
