<script>
  import Button from "../core/Button.svelte";
  import Tier from "./Tier.svelte";
  import { todoButtons } from "./todos.js";

  /** @type {{documents?: import("./todos.js").TodoDocument[], onOpenDocument?: (path: string) => void}} */
  let { documents = [], onOpenDocument } = $props();

  let buttons = $derived(todoButtons(documents));
</script>

{#if buttons.length}
  <Tier label="Todos" name="todos">
    <div class="buttons">
      {#each buttons as button (button.path)}
        <Button variant="outline" size="sm" data-todo-document={button.path} onclick={() => onOpenDocument?.(button.path)}>
          {button.title}
          {#if button.count}<span class="count">{button.count}</span>{/if}
        </Button>
      {/each}
    </div>
  </Tier>
{/if}

<style>
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8px * var(--ui-scale));
  }

  .count {
    color: var(--artifact-todo);
  }
</style>
