import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import { mount } from "svelte";
import PlacementOverlay from "../src/lib/placement/PlacementOverlay.svelte";

const query = new URLSearchParams(location.search);
const clear = query.has("clear");

const view = clear
  ? { misplaced: [], skipped: [], leavingNote: "", relaunchNote: "" }
  : {
      misplaced: [
        { slug: "kraken", name: "Kraken", state: "misplaced", from: { id: "work", path: "/work" }, to: { id: "default", path: "/d" }, leaving: true },
        { slug: "squid", name: "Squid", state: "misplaced", from: { id: "default", path: "/d" }, to: { id: "work", path: "/work" }, leaving: false },
      ],
      skipped: [{ slug: "octopus", name: "Octopus", state: "skipped", reason: "agent-running", from: { id: "", path: "" }, to: { id: "", path: "" } }],
      leavingNote: "Sync deletes the folder on your teammates' machines.",
      relaunchNote: query.has("relaunch") ? "A saved change is waiting for a relaunch." : "",
    };

window.moves = [];
window.closes = 0;
window.wailsCall = async (name, input) => {
  if (name.endsWith("Placement.Report")) return view;
  if (name.endsWith("Placement.Move")) {
    window.moves.push(input);
    return input.map((slug) =>
      slug === "squid" ? { slug, result: "conflict", detail: "already exists" } : { slug, result: "moved" },
    );
  }
  if (name.endsWith("Placement.Close")) {
    window.closes++;
    return undefined;
  }
  return undefined;
};

mount(PlacementOverlay, { target: document.querySelector("#fixture") });
