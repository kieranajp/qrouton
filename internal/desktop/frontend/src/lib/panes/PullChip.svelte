<script>
  import Icon from "../core/Icon.svelte";
  import { ICON_COLOURS } from "../core/icons.js";
  import { STATE_WORDS } from "../dashboard/dashboard.js";
  import { ICONS } from "../overview/pulls.js";
  import { openURL } from "../wails.js";

  /** @type {{ref: import("./todo.js").PullRef<import("../dashboard/dashboard.js").PullRequest>}} */
  let { ref } = $props();
</script>

<a
  class="pull"
  href={ref.url}
  title={ref.pull?.title ?? null}
  data-pull={ref.pull?.state ?? ""}
  onclick={(event) => (event.preventDefault(), openURL(ref.url))}>
  {#if ref.pull}
    <span
      class="pull-state"
      role="img"
      aria-label={STATE_WORDS[ref.pull.state]}
      title={STATE_WORDS[ref.pull.state]}
      style:color={ICON_COLOURS[ICONS[ref.pull.state]]}><Icon name={ICONS[ref.pull.state]} inherit /></span>
  {/if}
  {ref.pull?.repo ?? ref.repo}#{ref.number}
</a>

<style>
  .pull {
    display: inline-flex;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    padding: 2px calc(7px * var(--ui-scale));
    border: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-secondary);
    text-decoration: none;
  }

  .pull:hover {
    background: var(--wash-selected);
  }

  .pull-state {
    display: inline-flex;
  }

  .pull-state :global(.icon) {
    width: calc(13px * var(--ui-scale));
    height: calc(13px * var(--ui-scale));
  }
</style>
