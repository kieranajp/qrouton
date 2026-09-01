<script>
  import Button from "../core/Button.svelte";
  import ChoiceChips from "../forms/ChoiceChips.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import TextField from "../forms/TextField.svelte";
  import { folder } from "./steps.js";

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

<StepHeading title="What are you working on?">
  The session name can stay descriptive. The branch description keeps its folder and branch short.
</StepHeading>

<div class="pair">
  <TextField
    label="Name"
    bind:value={name}
    help="Shown in the session and given to the agent" />
  <TextField
    label="Branch description — optional"
    bind:value={branchDescription}
    placeholder="short change summary"
    help="Short phrase for the folder and branch"
    helpLiteral={folder(branch)} />
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
  label="Description"
  multiline
  bind:value={description}
  help={'The agent reads this first. Say what "done" looks like if you know.'} />

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
    gap: 16px;
  }
</style>
