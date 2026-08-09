<script>
  import CubeMark from "../core/CubeMark.svelte";

  /** @type {{title?: string, titleAlign?: 'left'|'center', mark?: boolean, lights?: boolean, width?: string, height?: string, agent?: boolean, toolbar?: import('svelte').Snippet, footer?: import('svelte').Snippet, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    title,
    titleAlign = "left",
    mark = true,
    lights = true,
    width,
    height,
    agent = false,
    toolbar,
    footer,
    children,
    ...rest
  } = $props();
</script>

<div class="frame" class:agent style:width style:height {...rest}>
  <div class="titlebar">
    {#if lights}
      <div class="lights">
        <span class="light close"></span>
        <span class="light min"></span>
        <span class="light zoom"></span>
      </div>
    {/if}
    {#if mark}<CubeMark size={20} />{/if}
    {#if titleAlign === "center"}
      <span class="title centred">{title}</span>
      <span class="balance"></span>
    {:else}
      <span class="title">{title}</span>
    {/if}
    {#if toolbar}<span class="toolbar">{@render toolbar()}</span>{/if}
  </div>

  <div class="content">{@render children?.()}</div>

  {#if footer}
    <div class="footer">{@render footer()}</div>
  {/if}
</div>

<style>
  .frame {
    background: var(--surface-app);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-window);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .agent {
    border-color: var(--border-default);
    box-shadow: var(--shadow-window-agent);
  }

  .titlebar {
    height: var(--h-titlebar);
    flex: none;
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    padding: 0 14px;
    gap: 14px;
  }

  .agent .titlebar {
    height: 40px;
  }

  .lights {
    display: flex;
    gap: 8px;
  }

  .light {
    width: 12px;
    height: 12px;
    border-radius: var(--radius-dot);
  }

  .close {
    background: var(--mac-close);
  }

  .min {
    background: var(--mac-min);
  }

  .zoom {
    background: var(--mac-zoom);
  }

  .title {
    font: var(--machine-md);
    color: var(--text-muted);
  }

  .centred {
    flex: 1;
    text-align: center;
    color: var(--text-primary);
  }

  .balance {
    width: 60px;
  }

  .toolbar {
    margin-left: auto;
    display: flex;
    gap: 8px;
  }

  .content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .footer {
    height: var(--h-form-footer);
    flex: none;
    background: var(--surface-chrome);
    border-top: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    padding: 0 16px;
  }

  .agent .footer {
    height: var(--h-footer);
  }
</style>
