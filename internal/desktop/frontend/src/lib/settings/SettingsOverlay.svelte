<script>
  import { onMount } from "svelte";
  import Dialog from "../assembly/Dialog.svelte";
  import Settings from "./Settings.svelte";
  import { settings } from "./settings.svelte.js";

  /** @type {{onClose: () => void}} */
  let { onClose } = $props();

  const panel = settings(() => onClose());

  // A Dialog focuses its layer on mount alone, so the one underneath needs it back.
  onMount(() => {
    const beneath = /** @type {HTMLElement | null} */ (document.activeElement);
    return () => beneath?.focus?.();
  });
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
    bind:linear={panel.form.linear}
    bind:stickerLabels={panel.form.stickerLabels}
    linearPath={panel.form.linearPath}
    fields={panel.fields}
    restartRequired={panel.restartRequired}
    onAddOrg={panel.add}
    onRemoveOrg={panel.remove}
    onQuit={panel.quitAndRelaunch} />
</Dialog>
