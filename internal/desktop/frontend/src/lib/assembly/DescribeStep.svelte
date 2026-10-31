<script>
  import Button from "../core/Button.svelte";
  import ChoiceChips from "../forms/ChoiceChips.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import TextField from "../forms/TextField.svelte";

  /** @type {{name?: string, branchDescription?: string, description?: string, ticket?: string, prefix?: string, prefixes?: string[], branch?: string, fetching?: boolean, hint?: {text: string, tone: 'muted'|'success'|'failed'}, onFetch?: () => void}} */
  let {
    name = $bindable(""),
    branchDescription = $bindable(""),
    description = $bindable(""),
    ticket = $bindable(""),
    prefix = $bindable(""),
    prefixes = [],
    branch = "",
    fetching = false,
    hint = { text: "", tone: "success" },
    onFetch,
  } = $props();
</script>

<StepHeading title="What are you working on?" />

<div class="pair">
  <TextField
    label="Name"
    bind:value={name} />
  <TextField
    label="Branch description — optional"
    bind:value={branchDescription}
    placeholder="short change summary" />
</div>

<TextField
  label="Ticket — optional"
  bind:value={ticket}
  valueVoice="literal"
  help={hint.text}
  helpTone={hint.tone}>
  {#snippet trailing()}
    <Button variant="secondary" onclick={onFetch} disabled={fetching}
      >{fetching ? "Fetching…" : "Fetch"}</Button>
  {/snippet}
</TextField>

<TextField
  label="Description — optional"
  multiline
  bind:value={description} />

<ChoiceChips
  label="Branch prefix"
  options={prefixes}
  value={prefix}
  help="Editing repos get branch"
  helpLiteral={branch}
  onSelect={(option) => (prefix = option)} />

<style>
  .pair {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: calc(16px * var(--ui-scale));
  }
</style>
