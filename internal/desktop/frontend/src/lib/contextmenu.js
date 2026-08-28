// What a right click offers, by what it landed on. Pure so node --test can
// reach it, which is the whole frontend harness.

// Narrower than the rail's menu: these labels are one or two words.
export const MENU_WIDTH = 168;

const paste = { label: "Paste", act: "paste" };
const selectAll = { label: "Select All", act: "selectAll" };

/**
 * itemsFor is the menu a right click opens. A click with nothing to offer gets
 * no items and opens no menu: inert chrome in a native window has none either,
 * and a menu that only ever says "Copy" is how a page admits it is a page.
 * @param {{kind?: string, selection?: string, writable?: boolean, linkKind?: "document"|"external"|"none"}} at
 */
export function itemsFor({ kind, selection = "", writable = true, linkKind } = {}) {
  const copy = { label: "Copy", act: "copy", disabled: !selection };
  switch (kind) {
    case "link": {
      // A link nothing can follow gets no item rather than one that silently
      // does nothing.
      const followable = linkKind === "external" || linkKind === "document";
      const items = followable ? [{ label: "Open Link", act: "open" }] : [];
      items.push({ label: "Copy Link", act: "copyLink" });
      return items;
    }
    case "terminal":
      return [copy, paste, "-", selectAll];
    case "field":
      return writable
        ? [{ label: "Cut", act: "cut", disabled: !selection }, copy, paste, "-", selectAll]
        : [copy, "-", selectAll];
    case "text":
      return selection ? [copy] : [];
    default:
      return [];
  }
}
