<script>
  import OverviewPane from "../src/lib/overview/OverviewPane.svelte";

  const repos = [
    { name: "acme/web", role: "editing", measured: true, commits: 2, insertions: 74, deletions: 35 },
    { name: "acme/payments", role: "editing", measured: true, commits: 0, insertions: 12, deletions: 4 },
    { name: "acme/docs", role: "reference", measured: true, commits: 0 },
  ];
  /** @typedef {import("../src/lib/overview/pulls.js").PullRequestList} PullRequestList */
  /** @typedef {import("../src/lib/overview/digest.js").SessionDigest} Digest */
  /** @type {{summary: import("../src/lib/overview/summary.js").SessionSummary, pullRequests: PullRequestList,
   * digest: Digest}} */
  let { summary, pullRequests, digest } = $props();
  let added = $state(0);
  window.overviewAdded = () => added;
</script>

<OverviewPane slug="fixture" active={true} root="/s/checkout" {summary} {pullRequests} {digest} {repos} onAddRepos={() => added++} />
