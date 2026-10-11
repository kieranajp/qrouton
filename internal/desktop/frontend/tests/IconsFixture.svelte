<script>
  import Icon from "../src/lib/core/Icon.svelte";
  import { ICONS } from "../src/lib/core/icons.js";
</script>

{#each Object.keys(ICONS) as name (name)}
  <Icon {name} />
{/each}
