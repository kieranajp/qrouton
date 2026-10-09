export const NARROW_WIDTH = 420;

/** A window is narrow when it holds fewer than NARROW_WIDTH unscaled pixels. */
export const isNarrow = (width, percent) => width < (NARROW_WIDTH * percent) / 100;
