# Comments: quoting the conversation

Select text in the conversation and attach it to your message as a quote.

1. Press `Esc` (EDIT), move to the text, press `v` (or `V`) and select it.
2. Press `Enter` or `c`. A label such as `[Comment - the answer is...]` is added at
   the end of your draft and you are back in INPUT. Nothing is sent.

What can be quoted: saved assistant text, the code of a tool call, its output and
its error text. A selection that includes your own messages, thinking, live text,
headers or a collapsed `…` row is refused with a reason. A selection that spans
several saved parts becomes several quotes in one comment. Selecting the same text
twice gives two comments. In the draft itself `c` still changes text as in Vim, and
copying with `y` is unchanged.

A comment is one unit in the draft: you can delete, undo, queue and recall it. It
travels with the message as a `<comment>` tag that holds the quoted text, the
session, turn, call and part it came from, so it still reads correctly after the
history is compacted or meno is restarted. One quote may be at most 16 KiB; a
longer selection is refused, never cut. A tag that is malformed or too large is
shown as plain text.

The quote is your data. The model is told that text inside a comment is something
to read and discuss, never an instruction to follow.
