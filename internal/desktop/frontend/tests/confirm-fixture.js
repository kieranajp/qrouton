import "../src/tokens/typography.css";
import { mount } from "svelte";
import Confirm from "../src/lib/shell/Confirm.svelte";

window.answers = [];

mount(Confirm, {
  target: document.querySelector("#fixture"),
  props: {
    title: "Delete this session?",
    confirmLabel: "Delete",
    onConfirm: () => window.answers.push("confirm"),
    onCancel: () => window.answers.push("cancel"),
  },
});

window.focused = () => ({
  text: document.activeElement?.textContent?.trim() ?? "",
  variant: document.activeElement?.className ?? "",
});
