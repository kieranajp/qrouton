import "../src/tokens/scale.css";
import { mount } from "svelte";
import DockedDocument from "../src/lib/DockedDocument.svelte";

mount(DockedDocument, {
  target: document.querySelector("#fixture"),
  props: { id: "document-1", active: true },
});
