<script>
  import { onMount } from "svelte";
  import { PRESENTER_NOTE, PRESENTER_NOTES_EVENT } from "../lib/bridge/generated.js";
  import { call, Call, Events } from "../lib/wails.js";
  import "../lib/panes/markdown.css";

  const NOTHING = { index: 0, total: 0, title: "", html: "" };

  let note = $state(NOTHING);
  let live = false;

  const taken = (data) => ({ ...NOTHING, ...(data ?? {}) });

  // Pulled as well as subscribed: a step dispatched before this page had a
  // listener would otherwise leave it blank until the presenter moved again.
  onMount(() => {
    const off = Events.On(PRESENTER_NOTES_EVENT, (event) => {
      live = true;
      note = taken(event.data);
    });
    call(Call.ByName(PRESENTER_NOTE)).then((answer) => {
      if (!live && answer.ok) note = taken(answer.value);
    });
    return off;
  });
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
    gap: 12px;
    padding: 0 14px 0 var(--w-traffic-lights);
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
    padding: 22px 26px 30px;
    color: var(--text-secondary);
  }
</style>
