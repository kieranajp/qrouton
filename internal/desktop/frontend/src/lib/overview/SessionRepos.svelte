<script>
  import { REPO_ROLES } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import { GLYPHS, READ_ONLY } from "../roles.js";

  /** @type {{repos?: any[], onAddRepos?: () => void}} */
  let { repos = [], onAddRepos } = $props();
</script>

<section class="tier session" aria-label="This session">
  <span class="label">This session</span>
  <div class="body">
    {#if repos.length}
      <div class="repos">
        {#each repos as repo (repo.name)}
          <div class="repo">
            <span class="repo-name {repo.role}">{GLYPHS[repo.role]} {repo.name}</span>
            {#if repo.role === REPO_ROLES.REFERENCE}
              <span class="stat">{READ_ONLY}</span><span></span>
            {:else if repo.measured === false}
              <span class="stat">unmeasured</span><span></span>
            {:else}
              <span class="stat">{repo.commits} commit{repo.commits === 1 ? "" : "s"}</span>
              <span class="diff"
                ><span class="added">+{repo.insertions ?? 0}</span>
                <span class="removed">−{repo.deletions ?? 0}</span></span>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <span class="empty">no repositories yet</span>
    {/if}
    <span class="add"><Button variant="cube" size="sm" glyph="+" wide onclick={onAddRepos}>Add repos</Button></span>
  </div>
</section>

<style>
  .tier {
    display: flex;
    align-items: baseline;
    gap: calc(10px * var(--ui-scale));
    min-width: 0;
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(12px * var(--ui-scale));
  }

  .session {
    margin-top: auto;
  }

  .label {
    flex: none;
    width: calc(108px * var(--ui-scale));
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
    white-space: nowrap;
    color: var(--text-faint);
  }

  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  .repos {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: baseline;
    gap: calc(6px * var(--ui-scale)) calc(16px * var(--ui-scale));
    font: var(--machine-xs);
  }

  .repo {
    display: contents;
  }

  .repo-name {
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

  .stat,
  .empty {
    color: var(--text-faint);
  }

  .empty {
    font: var(--machine-xs);
  }

  .diff {
    text-align: right;
  }

  .added {
    color: var(--state-success);
  }

  .removed {
    color: var(--state-failed);
  }

  .add {
    max-width: calc(200px * var(--ui-scale));
    padding: 0 calc(3px * var(--ui-scale)) calc(3px * var(--ui-scale)) 0;
  }

  /* Narrow, the label heads its content and a repository takes two lines. */
  @container overview (max-width: 27.5rem) {
    .tier {
      flex-direction: column;
      align-items: stretch;
      gap: calc(6px * var(--ui-scale));
    }

    .label {
      width: auto;
    }

    .repos {
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .repo {
      display: flex;
      flex-wrap: wrap;
      column-gap: calc(6px * var(--ui-scale));
      padding: calc(5px * var(--ui-scale)) 0;
    }

    .repo-name {
      flex-basis: 100%;
    }

    .stat,
    .diff {
      font-size: calc(9.5px * var(--ui-scale));
    }
  }
</style>
