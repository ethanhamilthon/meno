# Question Forms, TODO, and Acceptance Tracking

Part of [the implementation specification](../plan.md). Review draft.

## askUser API

    askUser({
      questions: [{id, text, type, options, recommendation}],
      timeout
    }) -> {answers: [{id, selected, text}]}

Types: single, multi, text. Options: [{id,label}]; no options for text questions.
IDs are unique nonempty strings in their respective scope. No old string overload.
Maximum 20 questions and 20 supplied choices each. Validate before showing any UI.
Timeout is finite seconds >=0; absent/0 waits until answer, cancellation, or exit.

Every question is required and always permits custom text. For single, a custom
Other answer is an alternative to one option. For multi, selected options and
Other text can coexist. Text questions require nonblank input. Do not count an
empty Other as an answer. Return answer IDs and selected option IDs in input order,
with custom/free text separate; unselected custom text must not leak into answers.
Recommendation is displayed context, not an auto-selected user answer.

## Question form UX

Show one question at a time, a compact question/status list, and progress count.
Preserve answers while navigating and resizing. Narrow terminals may collapse the
list to a selector but must retain access to every question and action.

Tab/Shift-Tab moves focus, Previous/Next moves questions, Space toggles a choice,
Enter activates the focused action. Vim motions apply to lists, not text entry.
Support mouse selection. Submit is available from every question; validate all
and focus the first unanswered question. Nothing is sent until explicit Submit.

Esc never closes the form; it may leave a nested focus mode without cancelling.
Only Cancel aborts. Cancel throws a typed AskCancelled exception without partial
answers, aborting uncaught remaining JS but not cancelling the main agent turn.
Timeout throws AskTimeout; parent cancellation propagates cancellation. Catchable
exceptions do not confer permission to re-ask the same rejected form automatically.

Human wait pauses JS execution timeout. Other sessions and child/task processing
continue; events for this parent queue until the form resolves. Persist submitted
question/answer data in history. Discard cancelled/timed-out unfinished drafts;
on restart mark an open form interrupted instead of automatically restoring it.

## TODO data and API

One active plan per session, with archived plan history. Stable IDs, ordered flat
items, no dependency graph or nesting. Only the owning agent changes it; users see
and collapse/expand but do not directly edit it.

    todo.list({includeArchived?}) -> {plan, items, criteria}
    todo.add({title, note?}) -> item
    todo.update(id, {title?, note?, status?, position?}) -> item
    todo.done(id, {note?}) -> item
    todo.remove(id) -> boolean
    todo.archive({reason}) -> archivedPlan
    todo.criteria.list() -> criterion[]
    todo.criteria.add({text}) -> criterion
    todo.criteria.update(id, {text?, status?, evidence?, agreementRef?}) -> criterion
    todo.criteria.remove(id, {reason}) -> boolean

Adding after archive starts a new plan with a new ID. Item statuses: pending,
in_progress, blocked, done, cancelled. Criterion statuses: not_checked, passed,
failed, deferred. Evidence is bounded descriptive text and/or stable record/artifact
references. Require evidence for passed/failed, reason and explicit user-agreement
reference for deferred. The application validates structure/reference existence;
it cannot certify a model's claim that tests passed or consent was sufficient.

Persist every mutation and version against the current execution boundary. Removal
creates an auditable tombstone; it cannot conceal unfinished acceptance. Changes
to scope/criteria must remain visible to continuation summaries and fork history.
Subagents own independent plans and cannot edit the parent's. The parent assesses
results before updating its own criteria.

## Completion and presentation

Render a compact collapsible plan/status view in the conversation UI without
mixing DB access into rendering. Show steps and criteria separately. Preserve the
user's collapsed preference; don't reopen on every token/update. It is not a new
user-editable task-management application.

Before completion, instruct the model to compare agreed scope, implementation,
and evidence. Failed/not_checked criteria mean Unverified/Blocked, not fully done.
Allow partial final reports and explanations; do not force an infinite execution
loop or forbid all final text. done steps do not automatically pass criteria.
Deferrals require user agreement, even though routine execution is autonomous.

## Acceptance

Test all question types, Other semantics, validation, navigation/resize/mouse,
Cancel/Esc/timeout/exit, event queueing, and no partial result leakage. Test TODO
persistence, archives, child isolation, evidence/status rules, and historical fork.
