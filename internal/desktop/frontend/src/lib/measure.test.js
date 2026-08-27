import assert from "node:assert/strict";
import test from "node:test";
import {
  createMeasurementAccumulator,
  createMeasurementController,
  serializedUTF8Bytes,
  startMeasurementHarness,
} from "./measure.js";

test("accumulator summarizes frame, task, event, exit and chrome metrics", () => {
  const measurement = createMeasurementAccumulator(100, 2);
  for (const interval of [40, 10, 21, 20]) measurement.frame(interval);
  measurement.supportsLongTasks();
  measurement.longTask(55);
  measurement.longTask(70);
  measurement.storageWrite("other");
  measurement.storageWrite("qrouton.human-pane:session-a");
  measurement.shellSplitterPointer("pointerdown");
  measurement.shellSplitterPointer("pointermove");
  measurement.shellSplitterPointer("pointermove");
  measurement.shellSplitterPointer("pointerup");
  measurement.event("window:data:b", 40, 3, 108);
  measurement.event("chrome:update", 20, 2, 112);
  measurement.event("chrome:update", 30, 4, 130);
  measurement.event("window:exit:b", 10, 1, 140);
  measurement.event("pty:exit:a", 11, 2, 145);

  assert.deepEqual(measurement.summarize(200, {
    terminalCount: 3,
    canvasCount: 2,
    viewport: { width: 1200, height: 800, devicePixelRatio: 2 },
  }), {
    durationMs: 100,
    frames: {
      enabled: true,
      count: 4,
      p50Ms: 20,
      p95Ms: 40,
      maxMs: 40,
      over20Count: 2,
      over20Percent: 50,
    },
    longTasks: { supported: true, count: 2, maxMs: 70, totalMs: 125 },
    storageWrites: 2,
    shellSplitter: {
      role: "separator",
      ariaLabel: "Resize the shell pane",
      pointerdown: 1,
      pointermove: 2,
      pointerup: 1,
      pointercancel: 0,
      storageWrites: 1,
    },
    exitCount: 2,
    exitedStreamCount: 2,
    duplicateExits: 0,
    expectedExits: 2,
    exitsComplete: true,
    terminalWrites: {
      count: 0,
      bytes: 0,
      completed: 0,
      pending: 0,
      totalMs: 0,
      maxMs: 0,
      lastCompleteMs: null,
      writesAfterExit: 0,
    },
    events: {
      "chrome:update": { count: 2, utf8Bytes: 50, dispatchTotalMs: 6, dispatchMaxMs: 4 },
      "pty:exit:a": { count: 1, utf8Bytes: 11, dispatchTotalMs: 2, dispatchMaxMs: 2 },
      "window:data:b": { count: 1, utf8Bytes: 40, dispatchTotalMs: 3, dispatchMaxMs: 3 },
      "window:exit:b": { count: 1, utf8Bytes: 10, dispatchTotalMs: 1, dispatchMaxMs: 1 },
    },
    firstChromeMs: 12,
    terminalCount: 3,
    canvasCount: 2,
    viewport: { width: 1200, height: 800, devicePixelRatio: 2 },
  });
});

test("duplicate exits cannot satisfy a multi-terminal drain", () => {
  const measurement = createMeasurementAccumulator(0, 2);
  measurement.event("pty:exit:a", 1, 1, 1);
  measurement.event("pty:exit:a", 1, 1, 2);

  const summary = measurement.summarize(3);
  assert.equal(summary.exitCount, 2);
  assert.equal(summary.exitedStreamCount, 1);
  assert.equal(summary.duplicateExits, 1);
  assert.equal(summary.exitsComplete, false);
});

test("serialized byte count uses JSON UTF-8 and tolerates cyclic input", () => {
  const event = { name: "pty:data:x", data: "a🥖é" };
  assert.equal(serializedUTF8Bytes(event), Buffer.byteLength(JSON.stringify(event), "utf8"));
  event.self = event;
  assert.equal(serializedUTF8Bytes(event), 0);
});

const controllerEnvironment = () => {
  let clock = 100;
  let nextFrame = 0;
  const frames = new Map();
  const canceledFrames = [];
  const observers = [];
  const writeCallbacks = [];

  class FakeStorage {
    setItem(key, value) {
      this[key] = value;
    }
  }

  class FakeObserver {
    static supportedEntryTypes = ["longtask"];

    constructor(callback) {
      this.callback = callback;
      this.disconnected = false;
      this.records = [];
      observers.push(this);
    }

    observe(options) {
      this.options = options;
    }

    disconnect() {
      this.disconnected = true;
    }

    takeRecords() {
      return this.records.splice(0);
    }
  }

  class FakeTerminal {
    write(_data, callback) {
      writeCallbacks.push(callback);
    }
  }

  const originalDispatch = function (event) {
    this.received.push(event);
    if (event.name.startsWith("pty:data:") || event.name.startsWith("window:data:")) {
      this.terminal.write(event.data);
    }
    clock += 3;
    return "dispatched";
  };
  const terminal = new FakeTerminal();
  const browser = {
    _wails: { dispatchWailsEvent: originalDispatch, received: [], terminal },
    innerWidth: 1440,
    innerHeight: 900,
    devicePixelRatio: 2,
    visualViewport: { width: 1400, height: 860, scale: 1 },
  };
  const document = {
    listeners: new Map(),
    addEventListener(type, listener) {
      const listeners = this.listeners.get(type) ?? [];
      listeners.push(listener);
      this.listeners.set(type, listeners);
    },
    removeEventListener(type, listener) {
      const listeners = this.listeners.get(type) ?? [];
      this.listeners.set(type, listeners.filter((candidate) => candidate !== listener));
    },
    dispatch(type, target, options = {}) {
      const event = { target, pointerId: 1, button: 0, ...options };
      for (const listener of this.listeners.get(type) ?? []) listener(event);
    },
    querySelectorAll(selector) {
      return { length: selector === ".xterm" ? 2 : selector === ".xterm canvas" ? 4 : 0 };
    },
  };
  const environment = {
    window: browser,
    document,
    performance: { now: () => clock },
    PerformanceObserver: FakeObserver,
    Storage: FakeStorage,
    Terminal: FakeTerminal,
    requestAnimationFrame(callback) {
      const id = ++nextFrame;
      frames.set(id, callback);
      return id;
    },
    cancelAnimationFrame(id) {
      canceledFrames.push(id);
      frames.delete(id);
    },
  };

  return {
    browser,
    canceledFrames,
    document,
    environment,
    FakeStorage,
    frames,
    observers,
    originalDispatch,
    originalTerminalWrite: FakeTerminal.prototype.write,
    setClock(value) {
      clock = value;
    },
    runFrame(timestamp) {
      const [id, callback] = frames.entries().next().value;
      frames.delete(id);
      callback(timestamp);
    },
    finishWrite() {
      writeCallbacks.shift()?.();
    },
  };
};

const separatorTarget = (label = "Resize the shell pane", role = "separator") => ({
  getAttribute(name) {
    return name === "role" ? role : name === "aria-label" ? label : null;
  },
});

test("controller patches, freezes, restores and can reset instrumentation", () => {
  const fixture = controllerEnvironment();
  const originalSetItem = fixture.FakeStorage.prototype.setItem;
  const controller = createMeasurementController(fixture.environment);
  const shellSplitter = separatorTarget();
  const decoySplitter = separatorTarget("Resize the shell panes");
  const wrongRole = separatorTarget("Resize the shell pane", "button");

  const storage = new fixture.FakeStorage();
  storage.setItem("width", "420");
  storage.setItem("qrouton.human-pane:alpha", "460");
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 2 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 2 });
  fixture.document.dispatch("pointerdown", wrongRole, { pointerId: 10 });
  fixture.document.dispatch("pointerup", wrongRole, { pointerId: 10 });
  fixture.document.dispatch("pointerdown", decoySplitter, { pointerId: 3 });
  fixture.document.dispatch("pointermove", decoySplitter, { pointerId: 3 });
  fixture.document.dispatch("pointerup", decoySplitter, { pointerId: 3 });
  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 4, button: 2 });
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 4 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 4 });
  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 5 });
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 6 });
  fixture.document.dispatch("pointermove", decoySplitter, { pointerId: 5 });
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 5 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 5 });
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 5 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 5 });
  fixture.setClock(105);
  assert.equal(
    fixture.browser._wails.dispatchWailsEvent.call(
      fixture.browser._wails,
      { name: "chrome:update", data: "🥖" },
    ),
    "dispatched",
  );
  fixture.runFrame(110);
  fixture.runFrame(127);
  fixture.runFrame(150);
  fixture.observers[0].callback({ getEntries: () => [{ duration: 61 }] });
  fixture.observers[0].records.push({ duration: 77 });
  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 12 });
  fixture.setClock(160);

  const stopped = controller.stop();
  assert.equal(stopped.storageWrites, 2);
  assert.deepEqual(stopped.shellSplitter, {
    role: "separator",
    ariaLabel: "Resize the shell pane",
    pointerdown: 2,
    pointermove: 1,
    pointerup: 1,
    pointercancel: 0,
    storageWrites: 1,
  });
  assert.deepEqual(stopped.frames, {
    enabled: true,
    count: 2,
    p50Ms: 17,
    p95Ms: 23,
    maxMs: 23,
    over20Count: 1,
    over20Percent: 50,
  });
  assert.deepEqual(stopped.longTasks, { supported: true, count: 2, maxMs: 77, totalMs: 138 });
  assert.equal(stopped.firstChromeMs, 5);
  assert.equal(stopped.events["chrome:update"].dispatchTotalMs, 3);
  assert.equal(stopped.terminalCount, 2);
  assert.equal(stopped.canvasCount, 4);
  assert.deepEqual(stopped.viewport.visual, { width: 1400, height: 860, scale: 1 });
  assert.equal(fixture.browser._wails.dispatchWailsEvent, fixture.originalDispatch);
  assert.equal(fixture.FakeStorage.prototype.setItem, originalSetItem);
  assert.deepEqual(
    [...fixture.document.listeners.values()].map((listeners) => listeners.length),
    [0, 0, 0, 0],
  );
  assert.equal(fixture.environment.Terminal.prototype.write, fixture.originalTerminalWrite);
  assert.equal(fixture.observers[0].disconnected, true);
  assert.equal(fixture.canceledFrames.length, 1);

  fixture.setClock(200);
  assert.equal(controller.reset(1), true);
  assert.notEqual(fixture.browser._wails.dispatchWailsEvent, fixture.originalDispatch);
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 12 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 12 });
  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 7 });
  fixture.document.dispatch("pointercancel", shellSplitter, { pointerId: 7 });
  fixture.browser._wails.dispatchWailsEvent({ name: "pty:data:x", data: new Uint8Array([1, 2, 3]) });
  fixture.browser._wails.dispatchWailsEvent({ name: "pty:exit:x", data: 0 });
  fixture.setClock(210);
  const draining = controller.snapshot();
  assert.equal(draining.exitsComplete, false);
  assert.deepEqual(draining.shellSplitter, {
    role: "separator",
    ariaLabel: "Resize the shell pane",
    pointerdown: 1,
    pointermove: 0,
    pointerup: 0,
    pointercancel: 1,
    storageWrites: 0,
  });
  assert.deepEqual(draining.terminalWrites, {
    count: 1,
    bytes: 3,
    completed: 0,
    pending: 1,
    totalMs: 0,
    maxMs: 0,
    lastCompleteMs: null,
    writesAfterExit: 0,
  });
  fixture.setClock(215);
  fixture.finishWrite();
  const drained = controller.snapshot();
  assert.equal(drained.exitsComplete, true);
  assert.equal(drained.terminalWrites.completed, 1);
  assert.equal(drained.terminalWrites.pending, 0);
  assert.equal(drained.terminalWrites.totalMs, 15);
  assert.equal(drained.terminalWrites.lastCompleteMs, 15);

  fixture.browser._wails.dispatchWailsEvent({ name: "pty:data:x", data: new Uint8Array([4]) });
  fixture.finishWrite();
  const outOfOrder = controller.snapshot();
  assert.equal(outOfOrder.exitsComplete, false);
  assert.equal(outOfOrder.terminalWrites.writesAfterExit, 1);

  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 8 });
  assert.equal(controller.reset(0, false), true);
  assert.equal(fixture.frames.size, 0);
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 8 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 8 });
  fixture.browser._wails.dispatchWailsEvent({ name: "chrome:update", data: null });
  const eventOnly = controller.snapshot();
  assert.equal(eventOnly.frames.enabled, false);
  assert.equal(eventOnly.events["chrome:update"].count, 1);
  assert.deepEqual(eventOnly.shellSplitter, {
    role: "separator",
    ariaLabel: "Resize the shell pane",
    pointerdown: 0,
    pointermove: 0,
    pointerup: 0,
    pointercancel: 0,
    storageWrites: 0,
  });
  fixture.document.dispatch("pointerdown", shellSplitter, { pointerId: 9 });
  const activeBeforeDestroy = controller.snapshot();
  assert.equal(activeBeforeDestroy.shellSplitter.pointerdown, 1);
  controller.destroy();
  assert.equal(fixture.browser._wails.dispatchWailsEvent, fixture.originalDispatch);
  fixture.document.dispatch("pointermove", shellSplitter, { pointerId: 9 });
  fixture.document.dispatch("pointerup", shellSplitter, { pointerId: 9 });
  storage.setItem("qrouton.human-pane:destroyed", "500");
  assert.deepEqual(controller.snapshot(), activeBeforeDestroy);
  assert.equal(fixture.document.listeners.get("pointerdown").length, 0);
  assert.equal(controller.reset(0), false);
});

test("harness is inert without a measurement URL", () => {
  assert.equal(startMeasurementHarness(""), undefined);
  assert.equal(startMeasurementHarness("https://127.0.0.1:4321"), undefined);
  assert.equal(startMeasurementHarness("ws://example.com:4321"), undefined);
  assert.equal(startMeasurementHarness("not a URL"), undefined);
});

test("harness speaks the control protocol and reconnects after socket loss", () => {
  const sockets = [];
  const timers = new Map();
  let nextTimer = 0;
  let clock = 50;
  let pagehide;
  let frameRequests = 0;

  class FakeStorage {
    setItem() {}
  }

  const originalDispatch = (_event) => "ok";
  const browser = {
    _wails: { dispatchWailsEvent: originalDispatch },
    innerWidth: 800,
    innerHeight: 600,
    devicePixelRatio: 1,
    addEventListener(name, listener) {
      if (name === "pagehide") pagehide = listener;
    },
    removeEventListener(name, listener) {
      if (name === "pagehide" && pagehide === listener) pagehide = undefined;
    },
  };

  class FakeWebSocket {
    static OPEN = 1;

    constructor(url) {
      this.url = url;
      this.readyState = 0;
      this.listeners = new Map();
      this.sent = [];
      sockets.push(this);
    }

    addEventListener(name, listener) {
      const listeners = this.listeners.get(name) ?? [];
      listeners.push(listener);
      this.listeners.set(name, listeners);
    }

    emit(name, event = {}) {
      for (const listener of this.listeners.get(name) ?? []) listener(event);
    }

    open() {
      this.readyState = FakeWebSocket.OPEN;
      this.emit("open");
    }

    send(message) {
      this.sent.push(JSON.parse(message));
    }

    message(command) {
      this.emit("message", { data: command });
    }

    close() {
      if (this.readyState === 3) return;
      this.readyState = 3;
      this.emit("close");
    }
  }

  const harness = startMeasurementHarness("ws://127.0.0.1:4321", {
    window: browser,
    document: { querySelectorAll: () => ({ length: 0 }) },
    performance: { now: () => clock, timeOrigin: 12345 },
    Storage: FakeStorage,
    Terminal: class {
      write(_data, callback) {
        callback?.();
      }
    },
    WebSocket: FakeWebSocket,
    requestAnimationFrame: () => ++frameRequests,
    cancelAnimationFrame: () => {},
    setTimeout(callback, delay) {
      const id = ++nextTimer;
      timers.set(id, { callback, delay });
      return id;
    },
    clearTimeout(id) {
      timers.delete(id);
    },
  });

  assert.equal(sockets[0].url, "ws://127.0.0.1:4321");
  assert.notEqual(browser._wails.dispatchWailsEvent, originalDispatch);
  sockets[0].open();
  assert.deepEqual(sockets[0].sent.shift(), { type: "ready", version: 1, timeOrigin: 12345 });

  sockets[0].message("not-json");
  assert.deepEqual(sockets[0].sent.shift(), { type: "error", message: "invalid JSON command" });
  sockets[0].message(JSON.stringify({
    type: "reset",
    id: 1,
    expectedExits: 1,
    trackFrames: false,
  }));
  assert.deepEqual(sockets[0].sent.shift(), { type: "ack", id: 1, command: "reset" });
  assert.equal(frameRequests, 1);
  browser._wails.dispatchWailsEvent({ name: "window:exit:x", data: 0 });
  clock = 70;
  sockets[0].message(JSON.stringify({ type: "snapshot", id: 2 }));
  const snapshot = sockets[0].sent.shift();
  assert.equal(snapshot.type, "result");
  assert.equal(snapshot.id, 2);
  assert.equal(snapshot.summary.exitCount, 1);
  assert.equal(snapshot.summary.exitsComplete, true);
  assert.equal(snapshot.summary.frames.enabled, false);

  sockets[0].message(JSON.stringify({ type: "stop", id: 3 }));
  assert.equal(sockets[0].sent.shift().type, "result");
  assert.equal(browser._wails.dispatchWailsEvent, originalDispatch);
  sockets[0].message(JSON.stringify({ type: "reset", id: 4, expectedExits: 0 }));
  assert.deepEqual(sockets[0].sent.shift(), { type: "ack", id: 4, command: "reset" });
  assert.notEqual(browser._wails.dispatchWailsEvent, originalDispatch);

  sockets[0].close();
  assert.equal(timers.size, 1);
  const [timerID, timer] = timers.entries().next().value;
  assert.equal(timer.delay, 1000);
  timers.delete(timerID);
  timer.callback();
  assert.equal(sockets.length, 2);
  sockets[1].open();
  assert.equal(sockets[1].sent.shift().type, "ready");
  sockets[1].message(JSON.stringify({ type: "destroy", id: 5 }));
  assert.deepEqual(sockets[1].sent.shift(), { type: "ack", id: 5, command: "destroy" });
  assert.equal(browser._wails.dispatchWailsEvent, originalDispatch);
  assert.equal(pagehide, undefined);
  assert.equal(timers.size, 0);
  assert.equal(sockets[1].readyState, 3);
  harness.destroy();
});
