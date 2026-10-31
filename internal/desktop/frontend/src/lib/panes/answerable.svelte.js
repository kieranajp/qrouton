import { untrack } from "svelte";
import { SvelteMap } from "svelte/reactivity";
import { freshContent, saveAnswers } from "./spec-calls.js";

/** The text drafts splice into, the drafts not yet saved, and each one's save state.
 * `splice` returns null once the draft's block is gone from the text.
 * @template D
 * @param {{id: () => string, doc: () => {text: string, hash?: string, viewportEpoch?: number}, splice: (text: string, key: string, draft: D) => string | null, reloaded: string}} of */
export function answerable({ id, doc, splice, reloaded }) {
  // The last push, or the pane's own save, or what it fetched after a refused one, whichever came last.
  let source = $state(untrack(() => ({ text: doc().text, hash: doc().hash ?? "" })));
  let epoch = $state(untrack(() => doc().viewportEpoch ?? 0));
  let pushed = untrack(() => doc().hash ?? doc().text);
  $effect(() => {
    const { text } = doc();
    const hash = doc().hash ?? "";
    const at = doc().viewportEpoch ?? 0;
    untrack(() => {
      if (at > epoch) epoch = at;
      if ((hash || text) === pushed) return;
      pushed = hash || text;
      source = { text, hash };
    });
  });

  /** @type {SvelteMap<string, D>} */
  const pending = new SvelteMap();
  /** @type {SvelteMap<string, {state: string, message?: string}>} */
  const saves = new SvelteMap();
  let queue = Promise.resolve();

  /** @param {string} key */
  async function save(key) {
    const draft = pending.get(key);
    if (draft === undefined) return;
    const text = splice(source.text, key, draft);
    if (text === null) return;
    if (text === source.text) {
      pending.delete(key);
      saves.delete(key);
      return;
    }
    const hash = source.hash;
    saves.set(key, { state: "saving" });
    const saved = await saveAnswers(id(), hash, text);
    if (saved.ok) {
      source = { text, hash: saved.value };
      if (pending.get(key) === draft) pending.delete(key);
      saves.set(key, { state: pending.has(key) ? "unsaved" : "saved" });
      return;
    }
    // Go refuses a stale hash; a fresh read tells that apart from a real failure.
    const fresh = await freshContent(id());
    if (fresh.ok && fresh.value.hash !== hash) {
      source = { text: fresh.value.text, hash: fresh.value.hash };
      epoch = Math.max(epoch, fresh.value.viewportEpoch ?? 0);
      saves.set(key, { state: "reloaded", message: reloaded });
      return;
    }
    saves.set(key, { state: "failed", message: `Could not save: ${saved.error?.message ?? saved.error}` });
  }

  return {
    get source() {
      return source;
    },
    get epoch() {
      return epoch;
    },
    pending,
    saves,
    /** @param {string} key @param {D} draft */
    put(key, draft) {
      pending.set(key, draft);
    },
    /** @param {string} key @param {D} draft */
    type(key, draft) {
      pending.set(key, draft);
      saves.set(key, { state: "unsaved" });
    },
    /** Saves run one at a time, so each splices into the text the last one left.
     * @param {string} key */
    commit(key) {
      if (!key || !pending.has(key)) return;
      queue = queue.then(() => save(key));
    },
    /** @param {string} key */
    discard(key) {
      pending.delete(key);
      saves.delete(key);
    },
    /** What still stands between the drafts and the file: a save in flight, one that did not land, or a typed draft. */
    get waiting() {
      const states = [...saves.values()].map((save) => save.state);
      if (states.includes("saving")) return "saving";
      if (states.includes("failed") || states.includes("reloaded")) return "failed";
      return pending.size > 0 ? "typed" : "";
    },
  };
}
