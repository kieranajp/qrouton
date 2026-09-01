<script>
  import DocumentIndex from "../src/lib/shell/DocumentIndex.svelte";
  import Menu from "../src/lib/shell/Menu.svelte";

  const items = [
    { tag: "PLAN", id: "P1", label: "plan.md" },
    { tag: "SPEC", id: "S1", label: "spec.md" },
    { tag: "RESEARCH", id: "R1", label: "research.md" },
    { tag: "NOTE", label: "note.md" },
  ];
  const repository = {
    label: "qrouton",
    items: [
      { tag: "PLAN", id: "P1", label: "plans/P1-in-repo.md", path: "src/qrouton/thoughts/plans/P1-in-repo.md" },
      { tag: "RESEARCH", id: "R1", label: "research/R1-history.md", path: "src/qrouton/thoughts/research/R1-history.md" },
    ],
  };
  const menuItems = [...items, "-", { heading: "In-repo" }, repository];
  const repositoryOnly = new URLSearchParams(window.location.search).has("repository-only");
</script>

{#if repositoryOnly}
  <DocumentIndex open onToggle={() => {}}>
    <Menu label="Written this session" items={["-", { heading: "In-repo" }, repository]} />
  </DocumentIndex>
{:else}
  <DocumentIndex count={items.length} open onToggle={() => {}}>
    <Menu label="Written this session" items={menuItems} onSelect={(item) => (window.selectedArtifact = item.path)} />
  </DocumentIndex>
{/if}
