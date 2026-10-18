# agents
- agents.spawn({task, cwd?, provider?, model?, effort?}) returns {id}. It starts a subagent and returns at once. provider, model and effort fall back to the default subagent settings, then to the values of this agent.
- agents.list(), agents.status(id), agents.cancel(id).
- agents.send(id, text) steers a running agent. It continues a completed or failed one and resumes a blocked one.
- agents.wait(id, {timeout?}) returns when the agent finishes, is blocked, or the timeout passes.
- agents.result(id) returns {id, status, result, error, usage}. result is null until it has one.
The subagent does not see this conversation. Put everything it needs in task: the goal, the context, the files, the constraints, and what to report. A restriction such as "do not edit files" is an instruction to the agent. The system does not enforce it. The subagent has the ordinary tools, but it has no askUser, no tasks, and no further subagents.
Statuses: starting, running, blocked, completed, failed, cancelled, interrupted. You get an event when an agent completes, fails, or blocks. Answer a blocked agent with agents.send.
You stay responsible for the work. Read what the agent did, check it against the agreed scope.
