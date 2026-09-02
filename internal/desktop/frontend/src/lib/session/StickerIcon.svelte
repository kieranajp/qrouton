<script>
  let { id, size = 14, outline = false } = $props();
</script>

<svg
  class="sticker-icon"
  width={size}
  height={size}
  viewBox="0 0 16 16"
  data-sticker={outline ? "none" : id}
  fill="none"
  aria-hidden="true">
  {#if id === "star"}
    <path
      d="m8 1.5 1.9 3.85 4.25.62-3.08 3 0.73 4.23L8 11.2l-3.8 2 .73-4.23-3.08-3 4.25-.62L8 1.5Z"
      fill={outline ? "none" : "currentColor"}
      stroke={outline ? "currentColor" : "none"}
      stroke-width={outline ? 1.3 : undefined}
      stroke-linejoin="round" />
  {:else if id === "bookmark"}
    <path d="M4 2.25h8v11.5L8 11.2l-4 2.55V2.25Z" fill="currentColor" />
  {:else if id === "question"}
    <path
      d="M5.25 5.35A2.8 2.8 0 0 1 8.1 2.7c1.7 0 2.9 1.05 2.9 2.55 0 1.22-.68 1.87-1.64 2.48-.84.53-1.11.9-1.11 1.72"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round" />
    <circle cx="8.25" cy="12.6" r="1.1" fill="currentColor" />
  {:else if id === "exclamation"}
    <path d="M8 2.5v7" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" />
    <circle cx="8" cy="12.6" r="1.1" fill="currentColor" />
  {/if}
</svg>

<style>
  .sticker-icon {
    display: block;
    flex: none;
  }
</style>
