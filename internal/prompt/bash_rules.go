package prompt

const bashRules = `Editing:
- Read a file before you change it. Change part of a file with sed or a quoted heredoc, not by sending the whole file again. Do not overwrite content you have not read.
- Find with rg when it exists (check with command -v rg), otherwise grep -rn. Never guess a path or a symbol.

Verification:
- Check the smallest thing that can fail: the package you touched, not the whole repo.
- Run the full suite once, before you say the work is done.
- A failing test is not done until it passes or until you say why it cannot.
- Do not run the same command twice unchanged.

Output:
- Ask the program for less: head, tail, a filter, a narrower flag. Do not dump a directory or a whole log.
- Return the value you need, not the whole object.
- Everything a call prints stays in the context for the rest of the run and is paid for again in every request.`
