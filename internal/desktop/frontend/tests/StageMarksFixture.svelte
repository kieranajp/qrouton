<script>
  import StageMarks from "../src/lib/shell/StageMarks.svelte";
</script>

<!-- R hollow, P filled, I hollow and live; then every mark filled and live. -->
<div data-row="mixed">
  <StageMarks stages={{ plan: true }} live={{ implement: true }} />
</div>
<div data-row="all">
  <StageMarks stages={{ research: true, plan: true, implement: true }} live={{ research: true, plan: true, implement: true }} />
</div>
