<script>
  import { REPO_ROLES } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import { GLYPHS, READ_ONLY } from "../roles.js";

  /** @type {{repos: any[], onAddRepos: () => void}} */
  let { repos, onAddRepos } = $props();
</script>

<div class="repos">
  <CapsLabel tone="dim" centred>This session</CapsLabel>
  {#if !repos.length}
    <div class="empty">no repositories yet</div>
  {/if}
  {#each repos as repo (repo.name)}
    <div class="repo">
      <div class="repo-name {repo.role}">
        {GLYPHS[repo.role]}
        {repo.name}
      </div>
      <div class="repo-stat">
        {#if repo.role === REPO_ROLES.REFERENCE}
          {READ_ONLY}
        {:else if repo.measured === false}
          unmeasured
        {:else}
          {repo.commits} commit{repo.commits === 1 ? "" : "s"}
          {#if repo.insertions || repo.deletions}
            · <span class="added">+{repo.insertions}</span>
            <span class="removed">−{repo.deletions}</span>
          {/if}
        {/if}
      </div>
    </div>
  {/each}
  <span class="allowance"></span>
  <Button variant="cube" size="sm" glyph="+" wide onclick={onAddRepos}>Add repos</Button>
</div>

<style>
  .repos {
    min-height: 0;
    padding-top: calc(10px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow: hidden auto;
  }

  .allowance {
    flex: none;
    height: calc(6px * var(--ui-scale));
  }

  .repo {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: calc(5px * var(--ui-scale)) 0;
  }

  .empty {
    font: var(--machine-xs);
    font-size: calc(10.5px * var(--ui-scale));
    color: var(--text-faint);
    padding: calc(5px * var(--ui-scale)) 0;
  }

  .repo-name {
    font: var(--machine-xs);
    font-size: calc(10.5px * var(--ui-scale));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .repo-name.editing {
    color: var(--role-editing);
  }

  .repo-name.reference {
    color: var(--role-reference);
  }

  .repo-stat {
    font: var(--machine-xs);
    font-size: calc(9.5px * var(--ui-scale));
    color: var(--text-faint);
  }

  .added {
    color: var(--state-success);
  }

  .removed {
    color: var(--state-failed);
  }
</style>
