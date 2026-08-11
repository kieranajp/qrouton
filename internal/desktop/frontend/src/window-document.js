import "./tokens/index.css";
import "./window-document.css";
import { mount } from "svelte";
import DocumentPane from "./lib/DocumentPane.svelte";

const id = new URLSearchParams(location.search).get("id");

mount(DocumentPane, { target: document.getElementById("document"), props: { id } });
