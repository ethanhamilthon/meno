# tasks start

Usage: meno tasks start CMD [--cwd DIR --env K=V --name NAME --no-notify]

Start CMD as a background process (a new shell, so quote it as one argument) and print its
task ID. A task has no time limit: it runs until it ends or you cancel it. --env may be
repeated. --name labels it in meno tasks list. A notice arrives when it finishes unless you
pass --no-notify. Follow it with meno tasks status, logs and wait.
