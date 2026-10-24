import { Call, call } from "../wails.js";

/** @param {import("./services.js").PTY} pty @param {string} id @param {number} cols @param {number} rows */
export const startPTY = (pty, id, cols, rows) => call(Call.ByName(pty.start, id, cols, rows));

/** @param {import("./services.js").PTY} pty @param {string} id @param {string} data */
export const writePTY = (pty, id, data) => call(Call.ByName(pty.write, id, data));

/** @param {import("./services.js").PTY} pty @param {string} id @param {number} cols @param {number} rows */
export const resizePTY = (pty, id, cols, rows) => call(Call.ByName(pty.resize, id, cols, rows));
