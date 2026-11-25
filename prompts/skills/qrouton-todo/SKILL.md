---
name: qrouton-todo
description: Internally keep the session's todo document, a short checklist of concrete actions only the human can take outside the conversation, such as merging a named pull request, setting an env var or secret, applying a named infrastructure change or confirming one thing in production. Never for decisions, conversations with other people, follow-ups or anything an agent can do or check. Use when work needs such an action, or when the user reports one done.
---

# Keep the todos for the user

The todo document lists what the user must do outside the conversation. It is not a stage, so never present it as one.

{{todo-bar}}

1. When work first needs an action that clears this bar, call `create_artefact` with `kind: todo` and a short title for the workstream. Keep one live document per workstream and add to it; start another only for separate work.
2. Write items in the order the user does them, under `##` sections. The shape is in `references/todo-shape.md` beside this file: the three states, the three fields and the done note. Read it before the first edit.
3. Tick an item only when you have verified the outcome yourself or the user says in chat that it is done. Never tick one because you expect it is done. Add a short note after ` — ` when ticking, such as the version that went live.
4. Mark an item `[-]` when it turns out not to be needed, and say why in its note.
5. Present the document with `open_file` after you create it or add items. The user ticks items in its pane, and its "Tell the agent" button types a fixed line into the conversation. When that line arrives, re-read the document from disk before acting on it.
