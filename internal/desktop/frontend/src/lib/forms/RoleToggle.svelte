<script>
  import SegmentedControl from "./SegmentedControl.svelte";

  /** @type {{key: 'off'|'editing'|'reference', label: string, accent: string, ink?: string}[]} */
  const ROLES = [
    { key: "off", label: "Off", accent: "var(--surface-raised)", ink: "var(--text-primary)" },
    { key: "editing", label: "Editing", accent: "var(--role-editing)" },
    { key: "reference", label: "Reference", accent: "var(--role-reference)" },
  ];

  /** offers is which roles this row will answer to; the rest render unanswering.
   * @type {{value?: 'off'|'editing'|'reference', offers?: ('off'|'editing'|'reference')[], onChange?: (role: string) => void, [attribute: string]: any}} */
  let { value = "off", offers = ["off", "editing", "reference"], onChange, ...rest } = $props();

  let segments = $derived(ROLES.map((role) => ({ ...role, disabled: !offers.includes(role.key) })));
</script>

<SegmentedControl {segments} {value} size="sm" onSelect={onChange} {...rest} />
