<script>
  import Menu from "../shell/Menu.svelte";
  import { MENU_WIDTH } from "../contextmenu.js";
  import { dismissible } from "../core/dismiss.js";
  import { menuHeight, place } from "../menu.js";
  import { copyImage, revealPath } from "../sessions.js";
  import { copyText } from "../wails.js";

  /** @type {{slug?: string, at: {image: {source: string, path?: string}, x: number, y: number}, onDismiss: () => void, onNotice?: (text: string) => void}} */
  let { slug = "", at, onDismiss, onNotice } = $props();

  // A gallery the workbench has no session for names no file on disk, so the
  // two items that reach for one are left out rather than drawn dead.
  let items = $derived(
    at.image.path && slug
      ? [
          { label: "Copy", act: "copy" },
          { label: "Copy Path", act: "copyPath" },
          { label: "Reveal", act: "reveal" },
        ]
      : [],
  );

  let anchor = $derived(
    place(
      at,
      { width: MENU_WIDTH, height: menuHeight(items) },
      { width: window.innerWidth, height: window.innerHeight },
    ),
  );

  async function act(item) {
    const path = at.image.path;
    onDismiss();
    try {
      switch (item.act) {
        case "copy":
          await copyImage(slug, path);
          onNotice?.("Image copied");
          break;
        case "copyPath":
          await copyText(path);
          onNotice?.("Path copied");
          break;
        case "reveal":
          await revealPath(slug, path);
          break;
      }
    } catch {
      onNotice?.("Could not " + item.label.toLowerCase() + ".");
    }
  }
</script>

{#if items.length}
  <div
    class="anchor"
    style:left="{anchor.left}px"
    style:top="{anchor.top}px"
    use:dismissible={onDismiss}>
    <Menu {items} width={MENU_WIDTH} offsetY={0} onSelect={act} />
  </div>
{/if}

<style>
  /* A zero-size point for the menu to resolve its own position against. */
  .anchor {
    position: fixed;
    width: 0;
    height: 0;
    z-index: 20;
  }
</style>
