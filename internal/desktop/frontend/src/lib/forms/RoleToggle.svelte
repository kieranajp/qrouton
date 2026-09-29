<script>
  import { REPO_ROLES } from "../bridge/generated.js";
  import SegmentedControl from "./SegmentedControl.svelte";

  /** @type {{key: "off" | import("../bridge/generated.js").RepoRole, label: string, accent: string, ink?: string}[]} */
  const ROLES = [
    { key: "off", label: "Off", accent: "var(--surface-raised)", ink: "var(--text-primary)" },
    { key: REPO_ROLES.EDITING, label: "Editing", accent: "var(--role-editing)" },
    { key: REPO_ROLES.REFERENCE, label: "Reference", accent: "var(--role-reference)" },
  ];

  /** offers is which roles this row will answer to; the rest render unanswering.
   * @type {{value?: "off" | import("../bridge/generated.js").RepoRole, offers?: ("off" | import("../bridge/generated.js").RepoRole)[], onChange?: (role: string) => void, [attribute: string]: any}} */
  let { value = "off", offers = ["off", REPO_ROLES.EDITING, REPO_ROLES.REFERENCE], onChange, ...rest } = $props();

  let segments = $derived(ROLES.map((role) => ({ ...role, disabled: !offers.includes(role.key) })));
</script>

<SegmentedControl {segments} {value} size="sm" onSelect={onChange} {...rest} />
