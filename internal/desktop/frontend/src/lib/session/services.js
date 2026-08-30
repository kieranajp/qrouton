import {
  PTY_DATA_EVENT,
  PTY_EXIT_EVENT,
  TERM_RESIZE,
  TERM_START,
  TERM_WRITE,
  WINDOW_DATA_EVENT,
  WINDOW_EXIT_EVENT,
  WINDOWS_RESIZE,
  WINDOWS_START,
  WINDOWS_WRITE,
} from "../bridge/generated.js";

/** @typedef {{start: string, write: string, resize: string, data: string, exit: string}} PTY */

/** @type {PTY} */
export const conversationPTY = {
  start: TERM_START,
  write: TERM_WRITE,
  resize: TERM_RESIZE,
  data: PTY_DATA_EVENT,
  exit: PTY_EXIT_EVENT,
};

/** @type {PTY} */
export const tabPTY = {
  start: WINDOWS_START,
  write: WINDOWS_WRITE,
  resize: WINDOWS_RESIZE,
  data: WINDOW_DATA_EVENT,
  exit: WINDOW_EXIT_EVENT,
};
