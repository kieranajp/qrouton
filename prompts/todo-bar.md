A todo item is one concrete action the user takes outside the conversation and can tick off: merge a named pull request, set an env var or secret, apply a named infrastructure change, run a named command where no agent can reach, or confirm one observable thing in production.

These are never items:

- a decision or judgement call, such as "decide whether" or "consider";
- a conversation with someone else, such as "check with" or "align with";
- a speculative follow-up, a nice-to-have or a ticket to file;
- anything an agent can do or check itself, such as tests, searches, `gh` queries or edits in an `editing` repo;
- a check the plan or CI already covers;
- one action split into several items.

Decisions and open questions belong in the conversation or the spec, never in the todo document. When in doubt, leave it out. A short list is the goal; an empty document is fine, and no document is better than a padded one.
