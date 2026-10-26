# tasks cancel

Usage: meno tasks cancel ID

Stop a task: its process group gets SIGTERM, then SIGKILL after a short grace time. The command
returns at once with the task as it was; run meno tasks status ID (or wait) to see it end as
cancelled. Cancelling a task that has finished does nothing.
