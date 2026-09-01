<script>
  import Button from "../core/Button.svelte";
  import "../diff.css";
  import { parsePatch } from "../diff.js";
  import { onDestroy, tick } from "svelte";
  import { activateMatch, clearMatches, createFindAdapter, markMatches } from "../find.js";
  import DiffRows from "./DiffRows.svelte";

  /** @typedef {{raw: true, inRaw: number} | {raw: false, file: any, inFile: number}} DiffFindMatch */

  /** @type {{doc: {text: string, format: string, source: string}, id?: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void, onFindAdapter?: (adapter: unknown) => void}} */
  let { doc, id: _id, active: _active, scrollRoot: _scrollRoot, onScroller, onFindAdapter } = $props();

  let patch = $derived(parsePatch(doc.text));
  let files = $derived(patch.files ?? []);
  let repositories = $derived(patch.repositories ?? []);
  let structured = $derived(patch.available);
  let mode = $state("files");
  let manualOpen = $state(new Set());
  let revealed = $state(/** @type {string | null} */ (null));
  let expandToken = 0;
  let findQuery = "";
  /** @type {HTMLElement | undefined} */
  let sheet = $state();

  $effect(() => {
    if (!structured || patch.confidence !== "full") mode = "raw";
  });

  $effect(() => {
    onScroller?.(sheet ?? null);
    return () => onScroller?.(null);
  });

  const diffFindAdapter = createFindAdapter({
    /** @returns {DiffFindMatch[]} */
    search: (query) => {
      findQuery = query;
      if (mode === "raw") return rawMatches(query);
      return fileMatches(query);
    },
    activate: async (matches, index) => {
      if (index < 0) return;
      const match = matches[index];
      if (match.raw === true) {
        activateMatch(markMatches(sheet, findQuery), match.inRaw);
        return;
      }
      revealed = match.file.id;
      await tick();
      const body = /** @type {HTMLElement | null} */ (sheet?.querySelector(`[data-file="${CSS.escape(match.file.id)}"] .diff-file-body`));
      const marked = markMatches(body, findQuery);
      activateMatch(marked, match.inFile);
    },
    reset: () => {
      clearMatches(sheet);
      revealed = null;
    },
  });

  $effect(() => {
    mode;
    onFindAdapter?.(null);
    onFindAdapter?.(diffFindAdapter);
  });

  function displayed(file) {
    return manualOpen.has(file.id) || revealed === file.id;
  }

  /** @param {MouseEvent} event @param {string} id */
  function toggle(event, id) {
    event.preventDefault();
    expandToken++;
    const next = new Set(manualOpen);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    manualOpen = next;
    if (revealed === id) revealed = null;
  }

  function collapseAll() {
    expandToken++;
    manualOpen = new Set();
    revealed = null;
  }

  function expandAll() {
    const token = ++expandToken;
    const add = (from) => {
      if (token !== expandToken) return;
      const next = new Set(manualOpen);
      for (const file of files.slice(from, from + 24)) next.add(file.id);
      manualOpen = next;
      if (from + 24 < files.length) requestAnimationFrame(() => add(from + 24));
    };
    add(0);
  }

  function showMode(next) {
    mode = next;
    if (next !== "files") revealed = null;
  }

  /** @returns {DiffFindMatch[]} */
  function fileMatches(query) {
    if (!query) return [];
    const expression = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "giu");
    const matches = [];
    for (const file of files) {
      let inFile = 0;
      for (const line of doc.text.slice(file.from, file.to).split(/\r?\n/)) {
        for (const _match of line.matchAll(expression)) matches.push({ raw: /** @type {false} */ (false), file, inFile: inFile++ });
      }
    }
    return matches;
  }

  /** @returns {DiffFindMatch[]} */
  function rawMatches(query) {
    if (!query) return [];
    const expression = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "giu");
    return [...doc.text.matchAll(expression)].map((_match, inRaw) => ({ raw: true, inRaw }));
  }

  function count(file, key) {
    const value = file[key];
    return Number.isSafeInteger(value) ? String(value) : "—";
  }

  function totals() {
    const { additions, deletions } = patch.totals ?? {};
    if (!Number.isSafeInteger(additions) || !Number.isSafeInteger(deletions)) return "Counts unavailable";
    return `+${additions} −${deletions}`;
  }

  function scope() {
    if (repositories.length > 1) return `${repositories.length} repositories inspected`;
    if (repositories.length === 1) return repositories[0].name || "this repository";
    return "this repository";
  }

  onDestroy(() => { expandToken++; onFindAdapter?.(null); });
</script>

<article class="diff-document">
  <div class="diff-sheet" bind:this={sheet}>
    {#if mode === "raw"}
      <pre class="diff-raw">{doc.text}</pre>
    {:else}
      <header class="diff-overview">
        <p class="diff-eyebrow">Change overview</p>
        <h1>{files.length} {files.length === 1 ? "file" : "files"}</h1>
        <p>{scope()} · {totals()}</p>
        {#if repositories.length > 1}
          <ul class="diff-repositories" aria-label="Inspected repositories">
            {#each repositories as repository (repository.id)}
              <li>{repository.name} · {repository.filesChanged ?? repository.fileIds?.length ?? 0} files</li>
            {/each}
          </ul>
        {/if}
        {#if patch.unassigned?.length}
          <p class="diff-notice">Raw patch contains additional output that is not shown as a file.</p>
        {/if}
      </header>

      {#if files.length}
        <div class="diff-items">
          {#each files as file (file.id)}
            <details class="diff-item" data-file={file.id} open={displayed(file)}>
              <summary onclick={(event) => toggle(event, file.id)}>
                <span class="diff-path">{file.path}</span>
                <span class="diff-status">{file.status}</span>
                <span class="diff-counts">+{count(file, "additions")} −{count(file, "deletions")}</span>
              </summary>
              {#if displayed(file)}
                <div class="diff-file-body"><DiffRows text={doc.text.slice(file.from, file.to)} /></div>
              {/if}
            </details>
          {/each}
        </div>
      {:else}
        <p class="diff-empty">No changes were reported for the inspected repositories.</p>
      {/if}
    {/if}
  </div>

  <footer class="diff-footer">
    <div class="diff-controls">
      {#if mode === "files" && structured}
        <div class="diff-bulk">
          <Button variant="ghost" size="sm" onclick={expandAll}>Expand all</Button>
          <Button variant="ghost" size="sm" onclick={collapseAll}>Collapse all</Button>
        </div>
      {/if}
      <div class="diff-modes">
        <Button variant={mode === "files" ? "outline" : "ghost"} size="sm" aria-pressed={mode === "files"} disabled={!structured} onclick={() => showMode("files")}>Files</Button>
        <Button variant={mode === "raw" ? "outline" : "ghost"} size="sm" aria-pressed={mode === "raw"} onclick={() => showMode("raw")}>Raw patch</Button>
      </div>
    </div>
  </footer>
</article>
