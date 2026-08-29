<script>
  import CubeMark from "../core/CubeMark.svelte";
  import { Events } from "../wails.js";
  import { STAGES, progressed, staged } from "./stages.js";

  // The gate has no button. The update is already downloading, and it applies
  // the moment nothing is running — so a dismiss here would be a control over
  // something the user cannot change, which is the sort of button that teaches
  // people to click past this screen.
  let stage = $state(staged());

  // The framework emits one payload per event, so it arrives as event.data
  // rather than as the first of a list.
  const advance = (name) => (event) => (stage = progressed(stage, name, event?.data));

  $effect(() => {
    const off = STAGES.map((name) => Events.On(name, advance(name)));
    return () => off.forEach((stop) => stop?.());
  });
</script>

<div class="layer" role="presentation">
  <div class="dialog">
    <CubeMark size={72} />
    <h1>Updating qrouton</h1>
    <p>
      This build is older than the release your team is on, so it can no longer start a session.
      qrouton is fetching the current version now and will restart into it on its own.
    </p>
    <div class="stage">
      <span class="dot" class:working={!stage.failed}></span>
      <span class="label">{stage.label}</span>
      {#if stage.percent > 0}
        <span class="percent">{stage.percent}%</span>
      {/if}
    </div>
    <div class="track" aria-hidden="true">
      <span class="fill" style:width="{stage.percent}%"></span>
    </div>
    <p class="foot">
      Sessions already running are untouched, and nothing here needs your attention. If this machine
      is offline the update waits, and <code>brew upgrade --cask qrouton</code> does the same job.
    </p>
  </div>
</div>

<style>
  /* Above the app's own ceiling of 5, and above the assembly overlay: an
     install the release feed has disowned may not start work behind this. */
  .layer {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--scrim);
  }

  .dialog {
    width: 620px;
    max-width: 100%;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
    padding: 34px;
    background: var(--surface-app);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-menu);
  }

  h1 {
    margin: 0;
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  p {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .stage {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .dot {
    width: 8px;
    height: 8px;
    background: var(--state-error, var(--accent-label));
  }

  .dot.working {
    background: var(--accent-action);
  }

  .label {
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .percent {
    font: var(--machine-sm);
    color: var(--text-faint);
  }

  .track {
    width: 100%;
    height: 3px;
    background: var(--border-subtle);
  }

  .fill {
    display: block;
    height: 100%;
    background: var(--accent-action);
    transition: width 120ms linear;
  }

  .foot {
    font: var(--machine-sm);
    color: var(--text-faint);
  }

  code {
    font: var(--machine-sm);
    color: var(--text-secondary);
  }
</style>
