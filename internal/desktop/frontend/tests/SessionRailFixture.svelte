<script>
  import { onMount } from "svelte";
  import Rail from "../src/lib/session/Rail.svelte";
  import { CHROME_EVENT } from "../src/lib/bridge/generated.js";
  import { DEFAULT_STICKER_LABELS } from "../src/lib/session/stickers.js";
  import { Events } from "../src/lib/wails.js";

  const first = {
    name: "Checkout migration",
    slug: "checkout",
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
  let stickerLabels = $state({ ...DEFAULT_STICKER_LABELS });

  onMount(() =>
    Events.On(CHROME_EVENT, (event) => {
      stickerLabels = { ...event.data.stickerLabels };
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

  window.sessionRail = {
    stateless,
    headline,
    shown: (slug) => (selected = slug),
    rename: (slug, name) => {
      sessions = sessions.map((session) => (session.slug === slug ? { ...session, name } : session));
    },
    stickerChanged: (slug, sticker) => {
      sessions = sessions.map((session) =>
        session.slug === slug ? { ...session, sticker } : session,
      );
    },
    setStickerLabels: (labels) => (stickerLabels = { ...stickerLabels, ...labels }),
  };
</script>

<button class="focus-probe" aria-label="Conversation focus">Conversation focus</button>
<div class="frame">
  <Rail
    {sessions}
    slug={selected}
    {stickerLabels}
    onNewSession={() => {}}
    onDismissed={() => {}} />
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
