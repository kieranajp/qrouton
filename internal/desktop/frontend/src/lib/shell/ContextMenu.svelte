<script>
  import { onMount } from "svelte";
  import Menu from "./Menu.svelte";
  import { MENU_WIDTH, itemsFor } from "../contextmenu.js";
  import { dismissible } from "../core/dismiss.js";
  import { menuHeight, place } from "../menu.js";
  import { openDocument } from "../docked.svelte.js";
  import { clipboardText, copyText, openURL } from "../wails.js";
  import { documentPath, linkKind } from "../panes/markdown.js";
  import { terminalAt } from "../xterm.js";

  /** @type {{kind: string, items: any[], x: number, y: number, [key: string]: any} | null} */
  let open = $state(null);

  let anchor = $derived(
    open
      ? place(
          open,
          { width: MENU_WIDTH, height: menuHeight(open.items) },
          { width: window.innerWidth, height: window.innerHeight },
        )
      : null,
  );

  // The terminal is asked first: its own textarea sits under the pointer, and a
  // field is what that would otherwise look like.
  function describe(target) {
    const term = terminalAt(target);
    if (term) return { kind: "terminal", term, selection: term.getSelection() };
    const field = target?.closest?.("input, textarea");
    if (field) {
      // The range is read here rather than acted on later: clicking the menu
      // blurs the field, and what a blurred field reports back is the engine's
      // business.
      const start = field.selectionStart ?? 0;
      const end = field.selectionEnd ?? 0;
      return {
        kind: "field",
        field,
        writable: !field.readOnly && !field.disabled,
        selection: String(field.value ?? "").slice(start, end),
        start,
        end,
      };
    }
    const link = target?.closest?.("a[href]");
    // getAttribute, not .href: the latter is DOM-resolved to an absolute URL,
    // which turns a relative document link into something linkKind reads as
    // pointing at the webview's own origin.
    if (link) {
      const href = link.getAttribute("href");
      const source = link.closest("[data-document-source]")?.getAttribute("data-document-source");
      const kind = linkKind(href);
      // A relative link is only followable from a pane that names the document
      // it is relative to.
      return { kind: "link", href, source, linkKind: kind === "document" && !source ? "none" : kind };
    }
    return { kind: "text", selection: String(window.getSelection() ?? "") };
  }

  function onContextMenu(event) {
    // A surface that drew a menu of its own has already claimed this click.
    if (event.defaultPrevented) return;
    // The webview draws one that names a browser nobody opened.
    event.preventDefault();
    const at = describe(event.target);
    const items = itemsFor(at);
    open = items.length ? { ...at, items, x: event.clientX, y: event.clientY } : null;
  }

  onMount(() => {
    window.addEventListener("contextmenu", onContextMenu);
    return () => window.removeEventListener("contextmenu", onContextMenu);
  });

  // setRangeText updates the field but leaves no event behind, and a binding
  // that never hears one keeps the text the user just replaced.
  function replace(at, text) {
    at.field.setRangeText(text, at.start, at.end, "end");
    at.field.dispatchEvent(new Event("input", { bubbles: true }));
  }

  async function paste(at) {
    const text = await clipboardText();
    if (!text) return;
    if (at.kind === "terminal") at.term.paste(text);
    else replace(at, text);
  }

  // The menu took the keyboard to be clicked; whatever was right-clicked wants
  // it back, and a paste with no cursor behind it goes nowhere.
  const restore = (at) => (at.kind === "terminal" ? at.term.focus() : at.field?.focus());

  async function act(item) {
    const at = open;
    open = null;
    restore(at);
    switch (item.act) {
      case "copy":
        await copyText(at.selection);
        break;
      case "cut":
        await copyText(at.selection);
        replace(at, "");
        break;
      case "paste":
        await paste(at);
        break;
      case "selectAll":
        if (at.kind === "terminal") at.term.selectAll();
        else at.field.select();
        break;
      case "open":
        if (at.linkKind === "document") openDocument(documentPath(at.href, at.source)).catch(() => {});
        else openURL(at.href);
        break;
      case "copyLink":
        await copyText(at.href);
        break;
    }
  }
</script>

{#if open}
  <div
    class="anchor"
    style:left="{anchor.left}px"
    style:top="{anchor.top}px"
    use:dismissible={() => (open = null)}>
    <Menu items={open.items} width={MENU_WIDTH} offsetY={0} onSelect={act} />
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
