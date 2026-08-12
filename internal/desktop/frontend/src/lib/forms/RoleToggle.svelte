<script>
  import SegmentedControl from "./SegmentedControl.svelte";

  const ROLES = [
    { key: "off", label: "Off", accent: "var(--surface-raised)", ink: "var(--text-primary)" },
    { key: "editing", label: "Editing", accent: "var(--role-editing)" },
    { key: "reference", label: "Reference", accent: "var(--role-reference)" },
  ];

  /** @type {{value?: 'off'|'editing'|'reference', disabled?: boolean, onChange?: (role: string) => void, [attribute: string]: any}} */
  let { value = "off", disabled = false, onChange, ...rest } = $props();
</script>

<SegmentedControl segments={ROLES} {value} {disabled} size="sm" onSelect={onChange} {...rest} />
