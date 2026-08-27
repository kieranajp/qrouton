import "./tokens/index.css";
import { mount } from "svelte";
import Session from "./Session.svelte";
import { startMeasurementHarness } from "./lib/measure.js";
import { Terminal as XtermTerminal } from "./lib/xterm.js";

const measurementURL = import.meta.env.VITE_QROUTON_MEASURE_URL;
const measurement = measurementURL
  ? startMeasurementHarness(measurementURL, { Terminal: XtermTerminal })
  : undefined;

mount(Session, { target: document.body });

if (import.meta.hot) import.meta.hot.dispose(() => measurement?.destroy());
