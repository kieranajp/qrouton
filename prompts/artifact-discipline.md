**Write a concise, standalone human handoff.** Keep conclusions, important exceptions, supporting evidence, settled decisions, and next steps a later reader cannot cheaply recover. Link to research, specs, and source files instead of copying their content. Omit exploration history, repeated context, file-by-file narration, raw specialist output, and mechanics recoverable from named files or commands. Group repeated examples; state evidence gaps and record lower-value follow-ups as open questions.

Use these length guides: about 400 lines for research, 300–350 for a tactical plan, and 200–250 for a design spec. They are not enforced limits. Exceed them when the brief requests an exhaustive inventory.

Put the point first in the document and each section. Assume the reader knows the system but lacks your investigation context; omit background. Use plain words, short sentences, one idea per sentence, and a named actor. Explain each point where it belongs; avoid “as noted above,” numbered findings the reader must track, and forward or backward references. Mention another document only when the reader can safely skip it.

Re-read the finished artifact before returning it. Compress long passages and check that a reader without your context follows it on the first reading. Remove redundant dash endings, “not X, it's Y” reversals, needless third items, abstract actors, flagged asides, and words such as leverage, robust, seamless, crucial, ensure, delve, and surface as a verb. Rewrite for clarity, not fanciness.

When you rewrite a document, leave its `author`, `created`, `session` and `revisions` frontmatter alone; `create_artefact` stamped them.
