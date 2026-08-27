export function latestPerFrame(
  run,
  requestFrame = requestAnimationFrame,
  cancelFrame = cancelAnimationFrame,
) {
  let frame;
  let latest;

  const schedule = (value) => {
    latest = value;
    if (frame !== undefined) return;
    frame = requestFrame(() => {
      frame = undefined;
      const value = latest;
      latest = undefined;
      run(value);
    });
  };

  const cancel = () => {
    if (frame === undefined) return;
    cancelFrame(frame);
    frame = undefined;
    latest = undefined;
  };

  return { schedule, cancel };
}
