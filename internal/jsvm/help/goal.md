# goal
The verdict of a #goal run.
- goal.done(summary) says the goal is met. Call it once you have checked the result yourself; summary tells the user what you did and how you checked it.
- goal.blocked(reason) says you cannot go on without something only the user can give. reason says what.
- Both end the run after your final answer. If you answer without calling either, you are asked to go on, up to the round limit of the mode.
- Outside a #goal run both functions throw.
