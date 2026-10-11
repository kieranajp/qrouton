<script>
  import ActivityPanel from "../session/ActivityPanel.svelte";

  /** @type {{agents?: any, active?: boolean}} */
  let { agents = {}, active = false } = $props();
</script>

<div class="agents" style:display={active ? "flex" : "none"}>
  <ActivityPanel {agents} />
</div>

<style>
  .agents {
    flex: 1;
    min-height: 0;
    flex-direction: column;
    overflow: hidden auto;
    padding: calc(12px * var(--ui-scale));
    background: var(--surface-app);
  }
</style>
