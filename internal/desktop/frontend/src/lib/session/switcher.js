function score(token, value) {
  value = String(value ?? "").toLowerCase();
  if (value === token) return 4000;
  if (value.startsWith(token)) return 3000;
  if (value.includes(token)) return 2000;
  let last = -1;
  let bonus = 0;
  for (const letter of token) {
    const at = value.indexOf(letter, last + 1);
    if (at < 0) return 0;
    bonus += (at === last + 1 ? 8 : 0) + (at === 0 || /[\s/_-]/.test(value[at - 1]) ? 8 : 0) - Math.min(8, at - last - 1);
    last = at;
  }
  return 1000 + Math.max(-200, Math.min(200, bonus));
}

export function matchSessions(rows = [], query = "") {
  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  const names = new Map();
  for (const row of rows) names.set(row.name, (names.get(row.name) ?? 0) + 1);
  return rows.map((row, order) => {
    const fields = [row.name, row.slug, ...(row.repos ?? []).map((repo) => repo.name)];
    let quality = 0;
    let priority = 0;
    const context = new Set(names.get(row.name) > 1 ? [row.slug] : []);
    for (const token of tokens) {
      const matches = fields.map((value, index) => ({ value, index, score: score(token, value) }));
      matches.sort((a, b) => b.score - a.score || a.index - b.index);
      const best = matches[0];
      if (!best.score) return null;
      quality += best.score;
      priority += Math.min(2, best.index);
      if (best.index === 1 || best.index > 2) context.add(best.value);
    }
    return { row, quality, priority, order, context: [...context].filter(Boolean).join(" · ") };
  }).filter((match) => match !== null).sort((a, b) => b.quality - a.quality || a.priority - b.priority || a.order - b.order);
}

export const initialSelection = (matches, current) => (matches.find(({ row }) => row.slug !== current) ?? matches[0])?.row.slug ?? "";
export const reconcileSelection = (matches, slug) => matches.some(({ row }) => row.slug === slug) ? slug : matches[0]?.row.slug ?? "";
