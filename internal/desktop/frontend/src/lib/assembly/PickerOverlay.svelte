<script>
  import Dialog from "./Dialog.svelte";
  import RepositoriesStep from "./RepositoriesStep.svelte";
  import { picking } from "./picker.svelte.js";
  import { joining } from "./steps.js";

  /** @type {{slug: string, escalating?: boolean, onClose: () => void}} */
  let { slug, escalating = false, onClose } = $props();

  const picker = picking(() => slug, () => onClose());
  const repos = picker.repos;

  let footer = $derived(picker.status || joining(picker.branch));
</script>

<Dialog
  secondary="Cancel"
  primary={escalating ? "Escalate →" : "Add repositories →"}
  status={footer}
  busy={picker.answering}
  onSecondary={picker.cancel}
  onPrimary={picker.confirm}
  onEscape={picker.cancel}>
  <RepositoriesStep
    bind:query={repos.query}
    orgs={repos.orgs}
    owners={repos.owners}
    failed={repos.failed}
    rows={repos.rows}
    shown={repos.shown}
    total={repos.total}
    tally={repos.tally}
    picks={repos.picks}
    refreshing={repos.refreshing}
    onOwner={repos.owner}
    onRefresh={repos.refetch}
    onRole={repos.role} />
</Dialog>
