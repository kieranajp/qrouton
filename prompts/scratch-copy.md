Work that calls for a scratch copy edits and runs only in the copy, never in the checkout you were given. A crash, a timeout or an interrupted run must leave that checkout exactly as it was.

1. Before you make the copy, record `git status --porcelain` for the checkout you were given, and a hash of its diff. Run them as `git -C <checkout> --no-optional-locks status --porcelain` and `git -C <checkout> --no-optional-locks diff | shasum`, so the reads themselves write nothing.
2. Make the copy: an empty directory from `mktemp -d`, filled by a plain recursive copy with no hard links that leaves out `.git`, `rsync -a --exclude=/.git <checkout>/ <copy>/`. Never use `cp -l`, `cp -al`, `rsync -H`, `rsync --link-dest`, `git worktree add`, `git stash`, `git checkout` or anything else that writes into the checkout or its `.git`.
3. Keep every edit and every working directory under the copy's path. Give each command the copy's absolute path, and start each shell in the copy. Before each edit, resolve the path with `realpath` and check that it starts with the copy's path, since a symbolic link in the copy can point back into the checkout.
4. Run no git command inside the copy. A checkout that is a git worktree has a `.git` file pointing back at the original repository, which is why the copy leaves `.git` out.
5. Before you change a file in the copy, save the original in a second `mktemp -d` directory, and restore it from there. No command ever names the checkout as a destination.
6. After the last run in the copy, take both reads of the checkout again and compare them with the first. Report any difference as a failure, with both outputs.
7. Delete the copy and the second directory when you are done.
