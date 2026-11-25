<script>
  import Icon from "../core/Icon.svelte";
  import { scrim } from "../core/dismiss.js";
  import { onMount, tick } from "svelte";
  import SessionIdentity from "./SessionIdentity.svelte";
  import SessionSticker from "./SessionSticker.svelte";
  import { rowLabel, summaryFacts } from "./rail.js";
  import { shownKey } from "./switcher.js";
  import { age } from "../relative.js";
  import { SWITCHER_LABEL, SWITCHER_SEARCH, SWITCHER_CLOSE, SWITCHER_EMPTY, SWITCHER_NO_MATCHES, SWITCHER_OPENING, SWITCHER_CURRENT, SWITCHER_RESULTS, SWITCHER_HELP } from "../bridge/generated.js";
  let { controller, current, stickers = [], total = 0 } = $props();
  let currentKey = $derived(shownKey(current));
  let sessions = $derived(controller.matches.filter((match) => !match.row.dashboard).length);
  let input;
  let panel;
  const optionID = (slug) => "session-choice-" + encodeURIComponent(slug);
  onMount(() => input.focus());
  $effect(() => {
    const active = controller.active;
    tick().then(() => document.getElementById(optionID(active))?.scrollIntoView({ block: "nearest" }));
  });
  $effect(() => { if (!controller.busy && controller.error) tick().then(() => input?.focus()); });
  function keydown(event) {
    if (event.key === "Tab") {
      const controls = [...panel.querySelectorAll("input, button")].filter((node) => !node.disabled);
      if (!controls.length) { event.preventDefault(); return; }
      const index = controls.indexOf(document.activeElement);
      event.preventDefault();
      controls[(index + (event.shiftKey ? -1 : 1) + controls.length) % controls.length].focus();
    }
    if (controller.busy || event.isComposing || event.keyCode === 229) return;
    if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); controller.cancel(); }
    if (event.target !== input) return;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); controller.move(event.key === "ArrowDown" ? 1 : -1); }
    if (event.key === "Enter") { event.preventDefault(); controller.activate(); }
  }
</script>

<div class="scrim" role="presentation" use:scrim={() => controller.cancel()}>
  <div class="switcher" role="dialog" aria-modal="true" aria-labelledby="session-switcher-title" aria-busy={controller.busy} bind:this={panel} onkeydown={keydown} tabindex="-1">
    <header><h2 id="session-switcher-title">{SWITCHER_LABEL}</h2><button type="button" aria-label={SWITCHER_CLOSE} disabled={controller.busy} onclick={controller.cancel}><Icon name="close" /></button></header>
    <label for="session-switcher-query">{SWITCHER_SEARCH}</label>
    <input id="session-switcher-query" bind:this={input} role="combobox" aria-autocomplete="list" aria-expanded="true" aria-controls="session-switcher-results" aria-activedescendant={controller.active ? optionID(controller.active) : undefined} readonly={controller.busy} value={controller.query} oninput={(event) => controller.query = event.currentTarget.value} />
    <div class="results" id="session-switcher-results" role="listbox" aria-label={SWITCHER_LABEL}>
      {#each controller.matches as match (match.row.slug)}
        {@const row = match.row}
        <div class="option" class:active={controller.active === row.slug} id={optionID(row.slug)} role="option" aria-selected={controller.active === row.slug} aria-label={[row.dashboard ? row.name : rowLabel(row.name, row.repos, summaryFacts(row.summary, row.unseen, "", row.todos)), match.context, row.slug === currentKey ? SWITCHER_CURRENT : ""].filter(Boolean).join(" · ")} onclick={() => controller.activate(row.slug)} onkeydown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); controller.activate(row.slug); } }} onmousedown={(event) => event.preventDefault()} tabindex="-1">
          {#if row.dashboard}
            <div class="home"><span class="tile"><Icon name="dashboard" inherit /></span><span class="title">{row.name}</span></div>
          {:else}
            <SessionIdentity initials={row.initials} name={row.name} repos={row.repos} summary={row.summary} unseen={row.unseen} todos={row.todos} idle={row.summary?.running ? "" : age(row.opened)} context={match.context} />
          {/if}
          {#if row.slug === currentKey}<span class="current">{SWITCHER_CURRENT}</span>{/if}
          {#if !row.dashboard}<SessionSticker id={row.sticker} {stickers} />{/if}
        </div>
      {/each}
    </div>
    <div role="status" aria-live="polite">{controller.busy ? SWITCHER_OPENING : !total ? SWITCHER_EMPTY : !controller.matches.length ? SWITCHER_NO_MATCHES : `${sessions} ${SWITCHER_RESULTS}`}</div>
    {#if controller.error}<div class="error" role="alert">{controller.error}</div>{/if}
    <footer>{SWITCHER_HELP}</footer>
  </div>
</div>
<style>
.scrim { position:fixed; inset:0; z-index:25; background:var(--scrim); display:flex; align-items:flex-start; justify-content:center; padding: min(12vh, calc(70px * var(--ui-scale))) calc(12px * var(--ui-scale)); box-sizing:border-box; }
.switcher { display:flex; flex-direction:column; gap:calc(10px * var(--ui-scale)); width:min(100%, calc(600px * var(--ui-scale))); max-height:100%; box-sizing:border-box; padding:calc(16px * var(--ui-scale)); background:var(--surface-chrome); border:1px solid var(--border-default); box-shadow:var(--shadow-focus); color:var(--text-primary); font:var(--machine-sm); }
header { display:flex; align-items:center; justify-content:space-between; }
h2 { margin:0; font:var(--display-xs); }
button { background:transparent; border:1px solid var(--border-subtle); color:inherit; font:inherit; cursor:pointer; }
input { width:100%; box-sizing:border-box; padding:calc(10px * var(--ui-scale)); background:var(--surface-app); color:inherit; border:1px solid var(--accent-action); font:inherit; }
.results { overflow:auto; min-height:0; display:flex; flex-direction:column; gap:calc(4px * var(--ui-scale)); }
.option { display:flex; align-items:center; gap:calc(10px * var(--ui-scale)); border:1px solid transparent; padding:calc(8px * var(--ui-scale)); cursor:pointer; }
.option.active { border-color:var(--accent-action); background:var(--surface-raised); }
.option:hover { background:var(--surface-raised); }
.current, footer { font:var(--machine-xs); color:var(--text-muted); }
.error { color:var(--state-failed); }
.home { display:flex; align-items:center; gap:calc(10px * var(--ui-scale)); min-width:0; flex:1; }
.tile { width:calc(30px * var(--ui-scale)); height:calc(30px * var(--ui-scale)); flex:none; display:flex; align-items:center; justify-content:center; border:1px solid var(--border-subtle); color:var(--accent-action); }
.title { font:var(--machine-md); font-size:calc(11px * var(--ui-scale)); color:var(--text-secondary); }
</style>
