For tells 1 to 3, apply the five-step check to each candidate:

1. Is there a tie: a type, a test, a generator or a build step that fails or regenerates when the copies drift?
2. Is there a reason written at the site that still holds?
3. Can the guarded case occur? Trace it to the producer of the value. A guard on an I/O boundary can always fire.
4. Do the copies agree today, and does each do what its own comment says?
5. Does a live caller in shipped code reach it? A caller in a test, a story or a fixture does not count.

The steps decide this way. Copies that disagree today are rot, whatever the intent. A copy that contradicts its own comment is rot only when the code is wrong or a reader acting on the comment would break something; a stale comment over correct code is a lone nit, and the copies still agree. Otherwise a copy with a tie, or with a written reason that still holds, is deliberate; a reason that no longer holds is rot. A tie that is only a test, pinning two copies together, is deliberate only behind a named boundary that rules out deriving one copy from the other; otherwise the copies fail step 1. A guard is rot unless a live producer can bring about the case it guards. Code that fails step 5, once the tool's traps are ruled out, is rot whatever the other steps say. Exported API that other modules consume counts as reached.
