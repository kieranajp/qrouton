let encoder;

const SHELL_SPLITTER_LABEL = "Resize the shell pane";
const SHELL_SPLITTER_STORAGE_PREFIX = "qrouton.human-pane:";
const SHELL_SPLITTER_POINTER_EVENTS = [
  "pointerdown",
  "pointermove",
  "pointerup",
  "pointercancel",
];

const percentile = (values, fraction) => {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((left, right) => left - right);
  return sorted[Math.max(0, Math.ceil(sorted.length * fraction) - 1)];
};

const maximum = (values) => values.reduce((largest, value) => Math.max(largest, value), 0);

export const serializedUTF8Bytes = (value) => {
  try {
    const serialized = JSON.stringify(value);
    if (serialized === undefined) return 0;
    encoder ??= new TextEncoder();
    return encoder.encode(serialized).byteLength;
  } catch {
    return 0;
  }
};

const utf8Bytes = (value) => {
  encoder ??= new TextEncoder();
  return encoder.encode(value).byteLength;
};

export function createMeasurementAccumulator(startedAt, expectedExits = 0, framesEnabled = true) {
  const frames = [];
  const longTasks = [];
  const events = new Map();
  const streams = new Map();
  let longTasksSupported = false;
  let storageWrites = 0;
  const shellSplitter = {
    role: "separator",
    ariaLabel: SHELL_SPLITTER_LABEL,
    pointerdown: 0,
    pointermove: 0,
    pointerup: 0,
    pointercancel: 0,
    storageWrites: 0,
  };
  let exitCount = 0;
  let exitedStreamCount = 0;
  let duplicateExits = 0;
  let firstChromeAt = null;
  let terminalWriteCount = 0;
  let terminalWriteBytes = 0;
  let terminalWriteCompleted = 0;
  let terminalWritePending = 0;
  let terminalWriteTotalMs = 0;
  let terminalWriteMaxMs = 0;
  let lastTerminalWriteAt = null;
  let writesAfterExit = 0;

  const streamKey = (name, kind) => {
    const prefix = `${kind}:`;
    return name?.startsWith(prefix) ? `${kind.split(":")[0]}:${name.slice(prefix.length)}` : undefined;
  };

  return {
    frame(interval) {
      if (Number.isFinite(interval) && interval >= 0) frames.push(interval);
    },
    longTask(duration) {
      if (Number.isFinite(duration) && duration >= 0) longTasks.push(duration);
    },
    supportsLongTasks() {
      longTasksSupported = true;
    },
    storageWrite(key) {
      storageWrites++;
      if (typeof key === "string" && key.startsWith(SHELL_SPLITTER_STORAGE_PREFIX)) {
        shellSplitter.storageWrites++;
      }
    },
    shellSplitterPointer(type) {
      if (SHELL_SPLITTER_POINTER_EVENTS.includes(type)) shellSplitter[type]++;
    },
    terminalWriteStarted(name, bytes) {
      const key = streamKey(name, "pty:data") ?? streamKey(name, "window:data");
      if (key) {
        const stream = streams.get(key) ?? { exited: false };
        if (stream.exited) writesAfterExit++;
        streams.set(key, stream);
      }
      terminalWriteCount++;
      terminalWriteBytes += Number.isFinite(bytes) && bytes >= 0 ? bytes : 0;
      terminalWritePending++;
    },
    terminalWriteFinished(duration, at) {
      terminalWriteCompleted++;
      terminalWritePending = Math.max(0, terminalWritePending - 1);
      const elapsed = Number.isFinite(duration) && duration >= 0 ? duration : 0;
      terminalWriteTotalMs += elapsed;
      terminalWriteMaxMs = Math.max(terminalWriteMaxMs, elapsed);
      if (Number.isFinite(at)) lastTerminalWriteAt = at;
    },
    event(name, utf8Bytes, dispatchDuration, at) {
      if (typeof name !== "string") return;
      const bytes = Number.isFinite(utf8Bytes) && utf8Bytes >= 0 ? utf8Bytes : 0;
      const duration = Number.isFinite(dispatchDuration) && dispatchDuration >= 0
        ? dispatchDuration
        : 0;
      const metric = events.get(name) ?? {
        count: 0,
        utf8Bytes: 0,
        dispatchTotalMs: 0,
        dispatchMaxMs: 0,
      };
      metric.count++;
      metric.utf8Bytes += bytes;
      metric.dispatchTotalMs += duration;
      metric.dispatchMaxMs = Math.max(metric.dispatchMaxMs, duration);
      events.set(name, metric);

      if (name === "chrome:update" && firstChromeAt === null) {
        firstChromeAt = Number.isFinite(at) ? at : startedAt;
      }
      const exited = streamKey(name, "pty:exit") ?? streamKey(name, "window:exit");
      if (exited) {
        exitCount++;
        const stream = streams.get(exited) ?? { exited: false };
        if (stream.exited) duplicateExits++;
        else exitedStreamCount++;
        stream.exited = true;
        streams.set(exited, stream);
      }
    },
    summarize(endedAt, environment = {}) {
      const over20Count = frames.filter((interval) => interval > 20).length;
      const eventSummary = Object.fromEntries(
        [...events.entries()].sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0),
      );

      return {
        durationMs: Math.max(0, endedAt - startedAt),
        frames: {
          enabled: framesEnabled,
          count: frames.length,
          p50Ms: percentile(frames, 0.5),
          p95Ms: percentile(frames, 0.95),
          maxMs: maximum(frames),
          over20Count,
          over20Percent: frames.length === 0 ? 0 : (over20Count / frames.length) * 100,
        },
        longTasks: {
          supported: longTasksSupported,
          count: longTasks.length,
          maxMs: maximum(longTasks),
          totalMs: longTasks.reduce((total, duration) => total + duration, 0),
        },
        storageWrites,
        shellSplitter: { ...shellSplitter },
        exitCount,
        exitedStreamCount,
        duplicateExits,
        expectedExits,
        exitsComplete: exitedStreamCount >= expectedExits
          && duplicateExits === 0
          && terminalWritePending === 0
          && writesAfterExit === 0,
        terminalWrites: {
          count: terminalWriteCount,
          bytes: terminalWriteBytes,
          completed: terminalWriteCompleted,
          pending: terminalWritePending,
          totalMs: terminalWriteTotalMs,
          maxMs: terminalWriteMaxMs,
          lastCompleteMs: lastTerminalWriteAt === null
            ? null
            : Math.max(0, lastTerminalWriteAt - startedAt),
          writesAfterExit,
        },
        events: eventSummary,
        firstChromeMs: firstChromeAt === null ? null : Math.max(0, firstChromeAt - startedAt),
        terminalCount: environment.terminalCount ?? 0,
        canvasCount: environment.canvasCount ?? 0,
        viewport: environment.viewport ?? { width: 0, height: 0, devicePixelRatio: 1 },
      };
    },
  };
}

const replaceMethod = (target, name, wrapper) => {
  if (!target) return undefined;
  const original = target[name];
  if (typeof original !== "function") return undefined;
  const descriptor = Object.getOwnPropertyDescriptor(target, name);
  const replacement = descriptor && "value" in descriptor
    ? { ...descriptor, value: wrapper }
    : {
        configurable: descriptor?.configurable ?? true,
        enumerable: descriptor?.enumerable ?? true,
        writable: true,
        value: wrapper,
      };

  try {
    Object.defineProperty(target, name, replacement);
  } catch {
    return undefined;
  }

  return () => {
    if (target[name] !== wrapper) return;
    try {
      if (descriptor) Object.defineProperty(target, name, descriptor);
      else delete target[name];
    } catch {}
  };
};

const browserEnvironment = (browser, documentObject) => {
  let terminalCount = 0;
  let canvasCount = 0;
  try {
    terminalCount = documentObject?.querySelectorAll(".xterm").length ?? 0;
    canvasCount = documentObject?.querySelectorAll(".xterm canvas").length ?? 0;
  } catch {}

  const viewport = {
    width: browser?.innerWidth ?? 0,
    height: browser?.innerHeight ?? 0,
    devicePixelRatio: browser?.devicePixelRatio ?? 1,
  };
  if (browser?.visualViewport) {
    viewport.visual = {
      width: browser.visualViewport.width,
      height: browser.visualViewport.height,
      scale: browser.visualViewport.scale,
    };
  }
  return { terminalCount, canvasCount, viewport };
};

export function createMeasurementController(environment = {}) {
  const browser = environment.window ?? globalThis.window;
  const documentObject = environment.document ?? browser?.document;
  const performanceObject = environment.performance ?? browser?.performance;
  const Observer = environment.PerformanceObserver ?? browser?.PerformanceObserver;
  const requestFrame = environment.requestAnimationFrame ?? browser?.requestAnimationFrame?.bind(browser);
  const cancelFrame = environment.cancelAnimationFrame ?? browser?.cancelAnimationFrame?.bind(browser);
  const StorageClass = environment.Storage ?? browser?.Storage;
  const TerminalClass = environment.Terminal;
  const now = () => performanceObject?.now?.() ?? 0;

  let accumulator;
  let running = false;
  let destroyed = false;
  let frozenSummary;
  let lastFrame = null;
  let frameID;
  let observer;
  let restoreStorage;
  let wailsPatch;
  let terminalPatch;
  let dispatchingEvent;
  let framesEnabled = true;
  let splitterListeners = [];
  const activeSplitterPointers = new Set();

  const patchStorage = () => {
    if (restoreStorage || !StorageClass?.prototype) return;
    const original = StorageClass.prototype.setItem;
    const wrapped = function (...args) {
      if (running) accumulator.storageWrite(args[0]);
      return Reflect.apply(original, this, args);
    };
    restoreStorage = replaceMethod(StorageClass.prototype, "setItem", wrapped);
  };

  const isShellSplitter = (target) => target?.getAttribute?.("role") === "separator"
    && target.getAttribute("aria-label") === SHELL_SPLITTER_LABEL;

  const observeShellSplitter = () => {
    if (splitterListeners.length || !documentObject?.addEventListener) return;
    for (const type of SHELL_SPLITTER_POINTER_EVENTS) {
      const listener = (event) => {
        const matches = isShellSplitter(event.target);
        if (type === "pointerdown") {
          if (!matches || event.button !== 0 || activeSplitterPointers.has(event.pointerId)) return;
          activeSplitterPointers.add(event.pointerId);
          if (running) accumulator.shellSplitterPointer(type);
          return;
        }
        if (!activeSplitterPointers.has(event.pointerId)) return;
        if (matches && running) accumulator.shellSplitterPointer(type);
        if (type === "pointerup" || type === "pointercancel") {
          activeSplitterPointers.delete(event.pointerId);
        }
      };
      documentObject.addEventListener(type, listener, true);
      splitterListeners.push({ type, listener });
    }
  };

  const stopObservingShellSplitter = () => {
    for (const { type, listener } of splitterListeners) {
      documentObject?.removeEventListener?.(type, listener, true);
    }
    splitterListeners = [];
    activeSplitterPointers.clear();
  };

  const restoreWails = () => {
    wailsPatch?.restore();
    wailsPatch = undefined;
  };

  const patchTerminal = () => {
    if (terminalPatch || !TerminalClass?.prototype) return;
    const original = TerminalClass.prototype.write;
    if (typeof original !== "function") return;
    const wrapper = function (data, callback) {
      if (!running) return Reflect.apply(original, this, [data, callback]);
      const target = accumulator;
      const startedAt = now();
      const bytes = typeof data === "string" ? utf8Bytes(data) : data?.byteLength ?? 0;
      target.terminalWriteStarted(dispatchingEvent, bytes);
      let finished = false;
      const finish = () => {
        if (finished) return;
        finished = true;
        const endedAt = now();
        target.terminalWriteFinished(Math.max(0, endedAt - startedAt), endedAt);
      };
      try {
        return Reflect.apply(original, this, [data, () => {
          try {
            finish();
          } finally {
            callback?.();
          }
        }]);
      } catch (error) {
        finish();
        throw error;
      }
    };
    const restore = replaceMethod(TerminalClass.prototype, "write", wrapper);
    if (restore) terminalPatch = { restore };
  };

  const patchWails = () => {
    const target = browser?._wails;
    const current = target?.dispatchWailsEvent;
    if (typeof current !== "function") return;
    if (wailsPatch?.target === target && current === wailsPatch.wrapper) return;
    restoreWails();

    const original = current;
    const wrapper = function (event) {
      let name;
      let bytes = 0;
      let startedAt = 0;
      if (running) {
        try {
          name = event?.name;
          bytes = serializedUTF8Bytes(event);
          startedAt = now();
        } catch {
          name = undefined;
        }
      }

      try {
        const previousEvent = dispatchingEvent;
        dispatchingEvent = name;
        try {
          return Reflect.apply(original, this, [event]);
        } finally {
          dispatchingEvent = previousEvent;
        }
      } finally {
        if (running && typeof name === "string") {
          const endedAt = now();
          accumulator.event(name, bytes, Math.max(0, endedAt - startedAt), startedAt);
        }
      }
    };
    const restore = replaceMethod(target, "dispatchWailsEvent", wrapper);
    if (restore) wailsPatch = { target, wrapper, restore };
  };

  const scheduleFrame = () => {
    if (!running || frameID !== undefined || !requestFrame) return;
    try {
      frameID = requestFrame((timestamp) => {
        frameID = undefined;
        if (!running) return;
        try {
          patchWails();
          if (lastFrame !== null) accumulator.frame(timestamp - lastFrame);
          lastFrame = timestamp;
        } finally {
          scheduleFrame();
        }
      });
    } catch {
      frameID = undefined;
    }
  };

  const observeLongTasks = () => {
    if (!Observer) return;
    const supported = Observer.supportedEntryTypes;
    if (Array.isArray(supported) && !supported.includes("longtask")) return;
    try {
      observer = new Observer((list) => {
        if (!running) return;
        for (const entry of list.getEntries()) accumulator.longTask(entry.duration);
      });
      observer.observe({ type: "longtask", buffered: false });
      accumulator.supportsLongTasks();
    } catch {
      observer?.disconnect?.();
      observer = undefined;
    }
  };

  const flushLongTasks = () => {
    if (!running || !observer?.takeRecords) return;
    try {
      for (const entry of observer.takeRecords()) accumulator.longTask(entry.duration);
    } catch {}
  };

  const teardownInstrumentation = () => {
    if (frameID !== undefined) {
      try {
        cancelFrame?.(frameID);
      } catch {}
      frameID = undefined;
    }
    observer?.disconnect?.();
    observer = undefined;
    restoreWails();
    terminalPatch?.restore();
    terminalPatch = undefined;
    restoreStorage?.();
    restoreStorage = undefined;
    stopObservingShellSplitter();
    lastFrame = null;
  };

  const snapshot = () => {
    if (!running) return frozenSummary;
    flushLongTasks();
    return accumulator.summarize(now(), browserEnvironment(browser, documentObject));
  };

  const reset = (expectedExits = 0, trackFrames = true) => {
    if (destroyed) return false;
    flushLongTasks();
    teardownInstrumentation();
    framesEnabled = trackFrames;
    accumulator = createMeasurementAccumulator(now(), expectedExits, framesEnabled);
    frozenSummary = undefined;
    running = true;
    patchStorage();
    observeShellSplitter();
    patchWails();
    patchTerminal();
    observeLongTasks();
    if (framesEnabled) scheduleFrame();
    return true;
  };

  const stop = () => {
    if (!running) return frozenSummary;
    flushLongTasks();
    frozenSummary = accumulator.summarize(now(), browserEnvironment(browser, documentObject));
    running = false;
    teardownInstrumentation();
    return frozenSummary;
  };

  const destroy = () => {
    if (destroyed) return;
    stop();
    destroyed = true;
  };

  reset();
  return { reset, snapshot, stop, destroy };
}

export function startMeasurementHarness(url, environment = {}) {
  if (!url) return undefined;

  try {
    const target = new URL(url);
    if (target.protocol !== "ws:" || !["127.0.0.1", "[::1]", "localhost"].includes(target.hostname)) {
      return undefined;
    }
  } catch {
    return undefined;
  }

  const browser = environment.window ?? globalThis.window;
  const performanceObject = environment.performance ?? browser?.performance;
  const WebSocketClass = environment.WebSocket ?? browser?.WebSocket;
  const setTimer = environment.setTimeout ?? browser?.setTimeout?.bind(browser);
  const clearTimer = environment.clearTimeout ?? browser?.clearTimeout?.bind(browser);
  if (!browser || !performanceObject || !WebSocketClass || !setTimer) return undefined;

  const controller = createMeasurementController({ ...environment, window: browser });
  let socket;
  let reconnectTimer;
  let destroyed = false;

  const send = (message, target = socket) => {
    try {
      if (target?.readyState === WebSocketClass.OPEN) target.send(JSON.stringify(message));
    } catch {}
  };

  const error = (message, id, target) => send({
    type: "error",
    ...(id === undefined ? {} : { id }),
    message,
  }, target);

  const onPageHide = () => destroy(undefined, socket, false);

  const destroy = (id, target = socket, acknowledge = true) => {
    if (destroyed) return;
    if (acknowledge) send({ type: "ack", id, command: "destroy" }, target);
    destroyed = true;
    if (reconnectTimer !== undefined) clearTimer?.(reconnectTimer);
    reconnectTimer = undefined;
    browser.removeEventListener?.("pagehide", onPageHide);
    controller.destroy();
    try {
      target?.close();
    } catch {}
  };

  const handle = (raw, target) => {
    let command;
    try {
      command = JSON.parse(raw);
    } catch {
      error("invalid JSON command", undefined, target);
      return;
    }
    if (!command || typeof command !== "object" || Array.isArray(command)) {
      error("command must be an object", undefined, target);
      return;
    }

    const { type, id } = command;
    if (id === undefined) {
      error("command id is required", undefined, target);
      return;
    }
    if (type === "reset") {
      if (!Number.isInteger(command.expectedExits) || command.expectedExits < 0) {
        error("expectedExits must be a non-negative integer", id, target);
        return;
      }
      if (command.trackFrames !== undefined && typeof command.trackFrames !== "boolean") {
        error("trackFrames must be a boolean", id, target);
        return;
      }
      controller.reset(command.expectedExits, command.trackFrames ?? true);
      send({ type: "ack", id, command: "reset" }, target);
      return;
    }
    if (type === "snapshot") {
      send({ type: "result", id, summary: controller.snapshot() }, target);
      return;
    }
    if (type === "stop") {
      send({ type: "result", id, summary: controller.stop() }, target);
      return;
    }
    if (type === "destroy") {
      destroy(id, target);
      return;
    }
    error("unknown command", id, target);
  };

  const connect = () => {
    if (destroyed) return;
    let next;
    try {
      next = new WebSocketClass(url);
    } catch {
      reconnectTimer = setTimer(() => {
        reconnectTimer = undefined;
        connect();
      }, 1000);
      return;
    }
    socket = next;
    next.addEventListener("open", () => {
      if (socket !== next) return;
      send({
        type: "ready",
        version: 1,
        timeOrigin: performanceObject.timeOrigin,
      }, next);
    });
    next.addEventListener("message", (event) => {
      if (socket !== next) return;
      try {
        handle(event.data, next);
      } catch {
        error("command failed", undefined, next);
      }
    });
    next.addEventListener("error", () => {
      if (socket !== next) return;
      try {
        next.close();
      } catch {}
    });
    next.addEventListener("close", () => {
      if (socket !== next) return;
      socket = undefined;
      if (!destroyed && reconnectTimer === undefined) {
        reconnectTimer = setTimer(() => {
          reconnectTimer = undefined;
          connect();
        }, 1000);
      }
    });
  };

  browser.addEventListener?.("pagehide", onPageHide, { once: true });
  connect();
  return { ...controller, destroy: () => destroy(undefined, socket, false) };
}
