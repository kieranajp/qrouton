<script>
  import { onMount } from "svelte";
  import { PRESENTER_NOTES_EVENT } from "../lib/bridge/generated.js";
  import { follow } from "../lib/wails.js";
  import { presenterNote } from "./calls.js";
  import "../lib/panes/markdown.css";

  const NOTHING = { index: 0, total: 0, title: "", html: "" };

  let note = $state(NOTHING);

  const taken = (data) => ({ ...NOTHING, ...(data ?? {}) });

  onMount(() => follow(PRESENTER_NOTES_EVENT, presenterNote, (data) => { note = taken(data); }, {
    settle: (answer, live) => { if (!live && answer.ok) note = taken(answer.value); },
  }).stop);

</script>

<div class="notes">
  <div class="band">
    {#if note.total}
      <span class="counter">{note.index + 1} / {note.total}</span>
    {/if}
    <span class="title">{note.title}</span>
  </div>
  <div class="body markdown">{@html note.html}</div>
</div>

<style>
  .notes {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--surface-app);
  }

  /* Every window this app opens hides the native title bar, so a page with no
     band of its own cannot be dragged. macOS draws its lights over the left. */
  .band {
    height: var(--h-titlebar);
    flex: none;
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
    padding: 0 calc(14px * var(--ui-scale)) 0 var(--w-traffic-lights);
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
    user-select: none;
    --wails-draggable: drag;
  }

  .counter {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .title {
    font: var(--display-xs);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: calc(22px * var(--ui-scale)) calc(26px * var(--ui-scale)) calc(30px * var(--ui-scale));
    color: var(--text-secondary);
  }
</style>
