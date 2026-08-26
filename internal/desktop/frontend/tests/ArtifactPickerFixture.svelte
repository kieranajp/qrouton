<script>
  import LatestDocument from "../src/lib/shell/LatestDocument.svelte";
  import Menu from "../src/lib/shell/Menu.svelte";

  const items = ["PLAN", "SPEC", "RESEARCH", "NOTE"].map((tag) => ({
    tag,
    label: `${tag.toLowerCase()}.md`,
  }));
</script>

<LatestDocument latest={{ tag: "PLAN", name: "plan.md", age: "now" }} count={items.length} open>
  <Menu label="Written this session" {items} />
</LatestDocument>
