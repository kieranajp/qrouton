Work that calls for a scratch copy edits and runs only in the copy, never in the checkout you were given. A crash, a timeout or an interrupted run must leave that checkout exactly as it was.

1. Before the first run, record `git status --porcelain` for the checkout you were given. Run it as `git -C <checkout> --no-optional-locks status --porcelain`, so the read itself writes nothing.
2. Make the copy: an empty directory from `mktemp -d`, filled by a plain recursive copy with no hard links, `cp -R <checkout>/. <copy>`. Never use `cp -l`, `cp -al`, `rsync --link-dest`, `git worktree add`, `git stash`, `git checkout` or anything else that writes into the checkout or its `.git`.
3. Keep every edit and every working directory under the copy's path. Give each command the copy's absolute path, and start each shell in the copy. Never edit through a symbolic link in the copy that points back into the checkout.
4. Run no git command inside the copy. A checkout that is a git worktree has a `.git` file pointing back at the original repository, so git in the copy writes there.
5. After the last run, record `git status --porcelain` for the checkout the same way and compare it with the first. Report any difference as a failure, with both outputs.
6. Delete the copy when you are done.
