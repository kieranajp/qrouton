<script>
  import LatestDocument from "../src/lib/shell/LatestDocument.svelte";
  import Menu from "../src/lib/shell/Menu.svelte";

  const items = ["PLAN", "SPEC", "RESEARCH", "NOTE"].map((tag) => ({
    tag,
    label: `${tag.toLowerCase()}.md`,
  }));
  const repository = {
    label: "qrouton",
    items: [
      { tag: "PLAN", label: "plans/P1-in-repo.md", path: "src/qrouton/thoughts/plans/P1-in-repo.md" },
      { tag: "RESEARCH", label: "research/R1-history.md", path: "src/qrouton/thoughts/research/R1-history.md" },
    ],
  };
  const menuItems = [
    ...items,
    "-",
    { heading: "In-repo" },
    repository,
  ];
  const repositoryOnly = new URLSearchParams(window.location.search).has("repository-only");
</script>

{#if repositoryOnly}
  <LatestDocument open onToggle={() => {}}>
    <Menu label="Written this session" items={["-", { heading: "In-repo" }, repository]} />
  </LatestDocument>
{:else}
  <LatestDocument latest={{ tag: "PLAN", name: "plan.md", age: "now" }} count={items.length} open>
    <Menu label="Written this session" items={menuItems} onSelect={(item) => (window.selectedArtifact = item.path)} />
  </LatestDocument>
{/if}
