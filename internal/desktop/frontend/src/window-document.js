import "./tokens/index.css";
import "./window-document.css";
import { Call } from "./lib/wails.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const id = new URLSearchParams(location.search).get("id");

// The window declares its format; nothing here guesses it from the text, or a
// plain document quoting a diff would be painted as one.
const META = ["diff --git", "index ", "--- ", "+++ ", "=== ",
  "new file", "deleted file", "rename ", "similarity ",
  "old mode", "new mode", "Binary files"];

const diffClass = (line) => {
  if (META.some((prefix) => line.startsWith(prefix))) return "file";
  if (line.startsWith("@@")) return "hunk";
  if (line.startsWith("+")) return "add";
  if (line.startsWith("-")) return "del";
  return "";
};

const doc = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
const body = document.getElementById("document");
for (const line of doc.text.split("\n")) {
  const row = document.createElement("div");
  row.className = doc.format === "diff" ? "row " + diffClass(line) : "row";
  row.textContent = line;
  body.appendChild(row);
}
