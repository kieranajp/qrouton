<script>
  import { onMount } from "svelte";
  import Rail from "../src/lib/session/Rail.svelte";
  import { CHROME_EVENT } from "../src/lib/bridge/generated.js";
  import { labelled } from "./session-rail-stickers.js";
  import { Events } from "../src/lib/wails.js";

  const first = {
    name: "Checkout migration",
    slug: "checkout",
    terminal: "conversation-checkout",
    initials: "CM",
    repos: [
      { name: "acme/web", role: "editing" },
      { name: "acme/a-very-long-editing-repository-name", role: "editing" },
    ],
    summary: { attention: "unread", active: 2, coverage: "full", running: true },
    unseen: 3,
    opened: new Date(Date.now() - 120000).toISOString(),
    sticker: "",
  };
  let sessions = $state([
    first,
    ...Array.from({ length: 11 }, (_, index) => ({
      name: `Session ${index + 2}`,
      slug: `session-${index + 2}`,
      initials: `S${index + 2}`,
      repos: [],
      summary: { attention: "none", active: 0, coverage: "none", running: false },
      unseen: 0,
      opened: new Date(Date.now() - (index + 1) * 86400000).toISOString(),
      sticker: "",
    })),
  ]);
  let selected = $state("checkout");
  let history = ["checkout"];
  /** @type {HTMLButtonElement | undefined} */
  let focusProbe;
  let stickers = $state(labelled({}));

  onMount(() =>
    Events.On(CHROME_EVENT, (event) => {
      stickers = event.data.stickers;
    }),
  );

  function stateless() {
    sessions = sessions.map((session) =>
      session.slug === "checkout"
        ? { ...session, summary: { attention: "unknown", active: 1, coverage: "full", running: true } }
        : session,
    );
  }

  function headline(summary) {
    sessions = sessions.map((session) =>
      session.slug === "checkout" ? { ...session, summary: { coverage: "full", running: true, ...summary } } : session,
    );
  }

  function load(slug) {
    sessions = sessions.map((session) =>
      session.slug === slug
        ? { ...session, terminal: `conversation-${slug}`, summary: { ...session.summary, running: true } }
        : session,
    );
  }

  window.sessionRail = {
    stateless,
    headline,
    shown: (slug) => {
      selected = slug;
      history = [slug, ...history.filter((item) => item !== slug)];
      load(slug);
    },
    load,
    order: () => sessions.map((row) => row.slug),
    remove: (slug) => (sessions = sessions.filter((row) => row.slug !== slug)),
    reordered: (slug, targetSlug) => {
      const source = sessions.findIndex((row) => row.slug === slug);
      const target = sessions.findIndex((row) => row.slug === targetSlug);
      if (source < 0 || target < 0) return;
      const next = [...sessions];
      const [row] = next.splice(source, 1);
      next.splice(target, 0, row);
      sessions = next;
    },
    unloaded: (slug) => {
      sessions = sessions.map((session) =>
        session.slug === slug
          ? {
              ...session,
              terminal: "",
              summary: { attention: "none", active: 0, coverage: "none", running: false },
            }
          : session,
      );
      if (selected === slug) {
        selected = history.find((item) => sessions.some((row) => row.slug === item && row.terminal)) ?? "";
      }
    },
    rename: (slug, name) => {
      sessions = sessions.map((session) => (session.slug === slug ? { ...session, name } : session));
    },
    stickerChanged: (slug, sticker) => {
      sessions = sessions.map((session) =>
        session.slug === slug ? { ...session, sticker } : session,
      );
    },
    setStickerLabels: (labels) =>
      (stickers = stickers.map((item) => ({ ...item, label: labels[item.id] ?? item.label }))),
  };
</script>

<button class="focus-probe" aria-label="Conversation focus" bind:this={focusProbe}>Conversation focus</button>
<div class="frame">
  <Rail
    {sessions}
    slug={selected}
    {stickers}
    onNewSession={() => {}}
    onDismissed={() => focusProbe?.focus()} />
</div>

<style>
  .frame {
    height: 100%;
    display: flex;
  }

  .focus-probe {
    position: fixed;
    left: -10000px;
  }
</style>
