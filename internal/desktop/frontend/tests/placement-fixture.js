import "../src/tokens/index.css";
import { mount } from "svelte";
import PlacementOverlay from "../src/lib/placement/PlacementOverlay.svelte";

const query = new URLSearchParams(location.search);
const clear = query.has("clear");

const skippedOnly = query.has("skippedonly");

const attention = query.has("attention");
const single = query.has("single");

const pendingRows = query.has("pending") || attention
  ? [{ slug: "crashed", name: "Crashed", state: "pending", detail: "Interrupted move could not be finished: permission denied", from: { id: "", path: "" }, to: { id: "", path: "" } }]
  : [];
const outside = query.has("outside");

const view = clear
  ? { misplaced: [], skipped: [], pending: pendingRows, leavingNote: "", relaunchNote: "" }
  : {
      misplaced: skippedOnly || attention ? [] : [
        { slug: "kraken", name: "Kraken", state: "misplaced", from: { id: "work", path: "/Users/me/Sync/lifesum" }, to: { id: "default", path: "/Users/me/work/thoughts" }, leaving: true },
        { slug: "squid", name: "Squid", state: "misplaced", from: { id: "default", path: "/Users/me/work/thoughts" }, to: { id: "work", path: "/Users/me/Sync/lifesum" }, leaving: false },
      ].slice(0, single ? 1 : 2),
      skipped: single ? [] : [
        outside
          ? { slug: "octopus", name: "Octopus", state: "skipped", reason: "outside-roots", from: { id: "", path: "/old/sync/octopus" }, to: { id: "", path: "" } }
          : { slug: "octopus", name: "Octopus", state: "skipped", reason: "agent-running", from: { id: "", path: "" }, to: { id: "", path: "" } },
      ],
      pending: pendingRows,
      leavingNote: skippedOnly || attention ? "" : "Moving a session out of a shared folder stops sharing it. Your sync tool will usually delete the folder on your teammates' machines, but versioned copies may remain.",
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
