export const macOS = /Macintosh|Mac OS X/.test(navigator.userAgent);

export function markPlatform(root = document.documentElement) {
  root.dataset.platform = macOS ? "macos" : "linux";
}
