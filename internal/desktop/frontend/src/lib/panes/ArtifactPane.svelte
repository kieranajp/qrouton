<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { artifactTone } from "../artifacts.js";
  import CopyPath from "./CopyPath.svelte";

  /** @type {{doc: {source: string, path?: string, kind?: string}, structured: string,
   * label: string, mode: string, onMode: (mode: string) => void, tag: any, body: any,
   * controls?: any, bar?: any, counter?: any}} */
  let { doc, structured, label, mode, onMode, tag, body, controls, bar, counter } = $props();
</script>

<article class="document">
  <div class="head">
    <CubeMark size={18} face={artifactTone(doc.kind)} data-artifact-kind={doc.kind ?? "NOTE"} />
    {@render tag()}
    {#if doc.source}
      <CapsLabel tone="dim">{doc.source}</CapsLabel>
    {/if}
    <CopyPath path={doc.path} />
  </div>
  {@render body()}
  <footer class="footer">
    {@render bar?.()}
    <div class="controls">
      {@render controls?.()}
      <div class="modes">
        <Button
          variant={mode === structured ? "outline" : "ghost"}
          size="sm"
          aria-pressed={mode === structured}
          onclick={() => onMode(structured)}>{label}</Button>
        <Button
          variant={mode === "document" ? "outline" : "ghost"}
          size="sm"
          aria-pressed={mode === "document"}
          onclick={() => onMode("document")}>Document</Button>
      </div>
      {@render counter?.()}
    </div>
  </footer>
</article>

<style>
  .document {
    --pane-pad: 34px;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  /* Aligned with the body's text column rather than the pane edge, so the
     mark, the chip and the path sit over the body's own left margin. */
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 26px var(--pane-pad) 20px calc(var(--pane-pad) + var(--gutter));
  }

  .head :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Held on the pane's floor whatever the body is tall enough to fill, so the
     controls stay under the same finger from one screen to the next. */
  .footer {
    flex: none;
    margin-top: auto;
    background: var(--surface-chrome);
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: var(--h-footer);
    padding: 0 var(--pane-pad);
  }

  .modes {
    display: flex;
    gap: 6px;
    margin-left: auto;
  }

  /* The counter is last in the markup but claims the slack, so the mode
     buttons sit against it rather than drifting with the body's width. */
  :global(.document > .footer .counter) {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: right;
    font: var(--machine-sm);
    color: var(--text-muted);
  }
</style>
