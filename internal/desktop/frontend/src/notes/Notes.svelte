<div class="notes">
  <div class="band"></div>
  <div class="body"></div>
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
    padding-left: var(--w-traffic-lights);
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
    user-select: none;
    --wails-draggable: drag;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 22px 26px 30px;
  }
</style>
