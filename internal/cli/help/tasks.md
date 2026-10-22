# tasks

Commands: start CMD [--cwd DIR] [--env K=V] [--name NAME] [--no-notify], list, status ID, logs ID [--from N] [--limit N], wait ID [--timeout SECONDS], cancel ID.

wait returns exit code 124 when its wait timeout expires. A timeout does not cancel the task.
