// A webview may hand back an alias's var() unresolved, so a token is followed to
// the literal it names and callers can ask for what a shade is for.
export const resolveToken = (read, name, depth = 0) => {
  const value = read(name).trim();
  const alias = /^var\((--[\w-]+)\)$/.exec(value);
  return alias && depth < 8 ? resolveToken(read, alias[1], depth + 1) : value;
};
