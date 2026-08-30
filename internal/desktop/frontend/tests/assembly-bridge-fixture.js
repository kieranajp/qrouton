import { mount } from "svelte";
import Overlay from "../src/lib/assembly/Overlay.svelte";

// ?fail=<method> makes that one bridge call reject and ?runners=none has it
// answer with nothing, which is how a workbench that cannot say what it runs is
// reproduced without a workbench.
const query = new URLSearchParams(location.search);
const failing = query.get("fail") ?? "";
const agents = query.get("runners") === "none" ? [] : [{ id: "claude", label: "Claude Code" }];

const calls = [];

window.wailsCall = async (name, ...args) => {
  calls.push({ name, args });
  if (failing && name.endsWith("." + failing)) throw new Error("assembly: no agent command");
  if (name.endsWith(".Begin")) return { ticket: "", entropy: "4f3a", generation: 3 };
  if (name.endsWith(".Prefixes")) return ["feat"];
  if (name.endsWith(".Runners")) return agents;
  if (name.endsWith(".Preview")) return "feat/thing-4f3a";
  if (name.endsWith(".Refresh")) return 0;
  return [];
};

mount(Overlay, { target: document.querySelector("#fixture"), props: { onClose: () => {} } });

window.bridge = {
  count: (method) => calls.filter(({ name }) => name.endsWith("." + method)).length,
};
