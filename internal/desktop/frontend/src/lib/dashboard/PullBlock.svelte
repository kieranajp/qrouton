<script>
  import Icon from "../core/Icon.svelte";
  import { ICON_COLOURS } from "../core/icons.js";
  import { ICONS } from "../overview/pulls.js";
  import { age } from "../relative.js";
  import { openURL } from "../wails.js";
  import { STATE_WORDS, pullTags } from "./dashboard.js";

  /** @type {{pr: import("./dashboard.js").PullRequest, now: number}} */
  let { pr, now } = $props();

  let reference = $derived(`${pr.repo}#${pr.number}`);
</script>

<a class="pull" href={pr.url} onclick={(event) => (event.preventDefault(), openURL(pr.url))}>
  <span class="row">
    <span class="state" style:color={ICON_COLOURS[ICONS[pr.state]]}><Icon name={ICONS[pr.state]} inherit />{STATE_WORDS[pr.state]}</span>
    {#each pullTags(pr) as tag (tag.label)}
      <span class="chip" style:color="var(--state-{tag.tone})">{tag.label}</span>
    {/each}
    <span class="repo" title={reference}>{reference}</span>
    <span class="when">{age(pr.updatedAt ?? "", now)}</span>
  </span>
  <span class="headline">{pr.title}</span>
</a>

<style>
  .pull {
    display: flex;
    flex-direction: column;
    gap: calc(5px * var(--ui-scale));
    padding: calc(9px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    color: var(--text-primary);
    text-decoration: none;
  }

  .pull:hover {
    background: var(--wash-selected);
  }

  .pull:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  /* A row too tight for its tags moves the repo and age down rather than clipping a tag. */
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(4px * var(--ui-scale)) calc(8px * var(--ui-scale));
    min-width: 0;
    font: var(--machine-xs);
  }

  .state {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: calc(5px * var(--ui-scale));
  }

  .state :global(.icon) {
    width: calc(13px * var(--ui-scale));
    height: calc(13px * var(--ui-scale));
  }

  .chip {
    flex: none;
    padding: 1px calc(4px * var(--ui-scale));
    border: 1px solid currentColor;
    font: 700 calc(9px * var(--ui-scale))/1.2 var(--font-machine);
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .repo {
    flex: 1 1 0;
    min-width: 4em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-secondary);
  }

  .when {
    flex: none;
    margin-left: auto;
    color: var(--text-faint);
  }

  .headline {
    font: var(--machine-bold);
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
  }

  .pull:hover .headline {
    text-decoration: underline;
  }
</style>
