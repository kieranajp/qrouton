<script>
  import "../diff.css";
  import { parseDiff } from "../diff.js";

  /** @type {{doc: {text: string, format: string, source: string}, id?: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id: _id, active: _active, scrollRoot: _scrollRoot, onScroller: _onScroller } = $props();

  let parsed = $derived(parseDiff(doc.text));
</script>

<div class="diff-grid" style={`--diff-gutter-width: ${parsed.digits + 1}ch`}>
  {#each parsed.rows as row, i (i)}
    <div class="diff-line diff-{row.kind}">
      <span
        class="diff-gutter diff-old"
        aria-hidden="true"
        data-line={row.oldLine ?? ""}
      ></span>
      <span
        class="diff-gutter diff-new"
        aria-hidden="true"
        data-line={row.newLine ?? ""}
      ></span>
      <span class="diff-content">
        {row.text}{#if row.text === ""}<br aria-hidden="true" />{/if}
      </span>
    </div>
  {/each}
</div>

<style>
  .diff-grid {
    padding: 12px 14px;
  }
</style>
