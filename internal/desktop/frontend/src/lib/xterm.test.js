import assert from "node:assert/strict";
import test from "node:test";
import { createTerminalPainter, encode } from "./terminal-painter.js";

const key = (kind, id) => [kind, id.prefix ?? "", id.intermediates ?? "", id.final].join(":");

function terminalDouble() {
  const handlers = new Map();
  const writes = [];
  const disposed = [];
  const register = (kind, id, callback) => {
    const idKey = key(kind, id);
    handlers.set(idKey, callback);
    return {
      dispose() {
        disposed.push(idKey);
        handlers.delete(idKey);
      },
    };
  };
  return {
    parser: {
      registerCsiHandler: (id, callback) => register("csi", id, callback),
      registerDcsHandler: (id, callback) => register("dcs", id, callback),
    },
    write(data, callback) {
      writes.push({ data, callback });
    },
    handlers,
    writes,
    disposed,
  };
}

function pendingTerminal() {
  const term = terminalDouble();
  term.complete = () => {
    const write = term.writes.find(({ callback }) => callback);
    assert.ok(write, "expected a pending write callback");
    const callback = write.callback;
    write.callback = undefined;
    callback();
  };
  return term;
}

const text = (data) => typeof data === "string" ? data : new TextDecoder().decode(data);
const handler = (term, kind, id) => term.handlers.get(key(kind, id));

test("the painter serializes live and replay chunks by write completion", () => {
  const term = pendingTerminal();
  const painter = createTerminalPainter(term);
  const decrqss = handler(term, "dcs", { intermediates: "$", final: "q" });

  painter.paint(encode("live-1"));
  painter.paint({ encoded: encode("replay-1"), replay: true });
  painter.paint(encode("live-2"));
  painter.paint({ encoded: encode("replay-2"), replay: true });

  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1"]);
  assert.equal(decrqss("m", []), false);

  term.complete();
  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1", "\x1bc"]);
  assert.equal(decrqss("m", []), true);

  term.complete();
  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1", "\x1bc", "replay-1"]);
  assert.equal(decrqss("m", []), true);

  term.complete();
  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1", "\x1bc", "replay-1", "live-2"]);
  assert.equal(decrqss("m", []), false);

  term.complete();
  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1", "\x1bc", "replay-1", "live-2", "\x1bc"]);
  assert.equal(decrqss("m", []), true);

  term.complete();
  term.complete();
  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["live-1", "\x1bc", "replay-1", "live-2", "\x1bc", "replay-2"]);
  assert.equal(decrqss("m", []), false);
});

test("the replay guards consume only xterm response requests", () => {
  const term = pendingTerminal();
  const painter = createTerminalPainter(term);
  painter.paint({ encoded: encode("retained"), replay: true });

  const cases = [
    [{ final: "c" }, [0], true],
    [{ final: "c" }, [1], false],
    [{ prefix: ">", final: "c" }, [0], true],
    [{ prefix: ">", final: "c" }, [1], false],
    [{ intermediates: "$", final: "p" }, [4], true],
    [{ prefix: "?", intermediates: "$", final: "p" }, [2004], true],
    [{ final: "n" }, [5], true],
    [{ final: "n" }, [6], true],
    [{ final: "n" }, [7], false],
    [{ prefix: "?", final: "n" }, [6], true],
    [{ prefix: "?", final: "n" }, [5], false],
    [{ final: "t" }, [18], true],
    [{ final: "t" }, [14, 2], false],
    [{ final: "t" }, [8, 30, 100], false],
  ];
  for (const [id, params, expected] of cases) {
    assert.equal(handler(term, "csi", id)(params), expected, JSON.stringify({ id, params }));
  }
  assert.equal(handler(term, "dcs", { intermediates: "$", final: "q" })("m", []), true);

  term.complete();
  term.complete();
  for (const [id, params] of cases) {
    assert.equal(handler(term, "csi", id)(params), false, JSON.stringify({ id, params }));
  }
  assert.equal(handler(term, "dcs", { intermediates: "$", final: "q" })("m", []), false);
  painter.dispose();
});

test("disposing clears queued output and makes write callbacks inert", () => {
  const term = pendingTerminal();
  const painter = createTerminalPainter(term);

  painter.paint({ encoded: encode("retained"), replay: true });
  painter.paint(encode("queued"));
  painter.dispose();
  term.complete();
  painter.paint(encode("ignored"));

  assert.deepEqual(term.writes.map(({ data }) => text(data)), ["\x1bc"]);
  assert.equal(term.handlers.size, 0);
  assert.equal(term.disposed.length, 8);
});
