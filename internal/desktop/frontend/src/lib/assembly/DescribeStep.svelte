<script>
  import Button from "../core/Button.svelte";
  import ChoiceChips from "../forms/ChoiceChips.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import TextField from "../forms/TextField.svelte";
  import { folder } from "./steps.js";

  /** @type {{name?: string, description?: string, ticket?: string, prefix?: string, prefixes?: string[], branch?: string, fetching?: boolean, hint?: {text: string, tone: 'muted'|'success'|'failed'}, onFetch?: () => void}} */
  let {
    name = $bindable(""),
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
  The name becomes the folder and the branch, so keep it short.
</StepHeading>

<div class="pair">
  <TextField
    label="Name"
    bind:value={name}
    help="Folder and branch will be named"
    helpLiteral={folder(branch)} />
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
</div>

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
