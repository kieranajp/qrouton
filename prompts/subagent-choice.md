**Choose subagent capability deliberately.**

Choose the model for each delegation when the runner supports an override; do not inherit the parent model by habit.

Default to the efficient tier (to keep fan-out costs from ballooning) when the expected result is explicit and internally consistent, and the worker's job is faithful application: following an established pattern, making a specified mechanical change, performing a bounded lookup, or writing to a defined test shape.

Use the stronger reasoning tier when the worker must make a judgment: reconcile conflicting or incomplete sources, design a boundary or type, choose among plausible product or content answers, diagnose a systematic cause, or review work where a plausible mistake could survive unnoticed.

The test is: can the brief state the expected shape, and would a wrong result fail deterministic verification or be conspicuous in review? If so, use the efficient tier. If several reasonable interpretations remain, or a wrong answer could look correct and land silently, use the stronger tier.

Task size and importance are not capability signals. A large mechanical change may suit the efficient tier; a small ambiguous decision may require the stronger one. If you cannot name the judgment the stronger model must make, use the efficient tier.

If an efficient-tier worker discovers ambiguity or conflicting authority, it should return the conflict rather than silently resolve it; the parent can decide or re-delegate with stronger reasoning.
