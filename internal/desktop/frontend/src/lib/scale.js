export const NARROW_WIDTH = 420;

/** A window is narrow when it holds fewer than NARROW_WIDTH unscaled pixels. */
export const isNarrow = (width, percent) => width < (NARROW_WIDTH * percent) / 100;

/** The terminal font size at a scale, to the half pixel so xterm's cell stays stable. */
export const terminalSize = (base, percent) => Math.round((base * percent) / 50) / 2;
