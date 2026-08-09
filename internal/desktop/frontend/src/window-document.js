import "./tokens/index.css";
import "./lib/diff.css";
import "./window-document.css";
import { diffClass } from "./lib/diff.js";
import { Call } from "./lib/wails.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const id = new URLSearchParams(location.search).get("id");

// The window declares its format; guessing it from the text would paint a plain
// document that quotes a diff as one.
const doc = await Call.ByName(WINDOWS_SERVICE + ".Content", id);
const body = document.getElementById("document");
for (const line of doc.text.split("\n")) {
  const row = document.createElement("div");
  row.className = doc.format === "diff" ? "row " + diffClass(line) : "row";
  row.textContent = line;
  body.appendChild(row);
}
