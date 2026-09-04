<script>
  import { renderDeck } from "./slides.js";

  /** @type {{doc: {text: string, source: string, path?: string, kind?: string}}} */
  let { doc } = $props();

  let rendered = $derived(renderDeck(doc.text));
</script>

<svelte:head>
  {@html `<style>${rendered.css}</style>`}
</svelte:head>

<article class="deck">
  {@html rendered.html}
</article>

<style>
  .deck {
    padding: 26px 34px;
  }
</style>
