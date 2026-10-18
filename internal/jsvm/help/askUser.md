# askUser
askUser({questions: [{id, text, type, options?, recommendation?}], timeout?}) shows all questions at once as one form. It returns {answers: [{id, selected, text}]} in the same order.
- type is "single", "multi", or "text". single and multi need options [{id, label}], at most 20. text takes none.
- Ids are unique and not empty. Use at most 20 questions. Every question is required. The user can always add custom text.
- For single, custom text is an alternative to one option. Then selected is empty and text holds it.
- For multi, selected options and text can both be present. For text, text is the answer.
- recommendation is only shown to the user. It is never selected for them.
- timeout is in seconds. 0 or missing waits as long as it takes.
If the user cancels, askUser throws AskCancelled. If the time runs out, it throws AskTimeout. Catch them if you can go on. Do not ask the same rejected form again.
When to ask:
- Ask only when you cannot find the answer by reading files, running commands, or searching the project.
- Never ask permission for an ordinary action.
- Put related questions in one form. Keep the questions short. Give options when there are few.
