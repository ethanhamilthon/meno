# tasks wait

Usage: meno tasks wait ID [--timeout SECONDS]

Block until the task ends and print its ID and final status. With --timeout the wait ends
after that many seconds with exit code 124 and the task keeps running; a timeout does not
cancel it. Without --timeout it waits as long as the task runs.
