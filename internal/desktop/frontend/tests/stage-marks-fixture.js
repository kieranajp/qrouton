import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import StageMarksFixture from "./StageMarksFixture.svelte";

mount(StageMarksFixture, { target: document.querySelector("#fixture") });
