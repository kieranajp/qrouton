<script>
  import Dialog from "../assembly/Dialog.svelte";
  import Settings from "./Settings.svelte";
  import { settings } from "./settings.svelte.js";

  /** @type {{onClose: () => void}} */
  let { onClose } = $props();

  const panel = settings(() => onClose());
</script>

<Dialog
  secondary="Cancel"
  primary="Save"
  status={panel.status}
  busy={panel.saving}
  onSecondary={panel.cancel}
  onPrimary={panel.save}
  onEscape={panel.cancel}>
  <Settings
    orgs={panel.form.orgs}
    bind:orgInput={panel.orgInput}
    bind:root={panel.form.root}
    bind:editor={panel.form.editor}
    bind:launch={panel.form.launch}
    fields={panel.fields}
    restartRequired={panel.restartRequired}
    onAddOrg={panel.add}
    onRemoveOrg={panel.remove}
    onQuit={panel.quitAndRelaunch} />
</Dialog>
