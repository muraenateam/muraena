// Pure predicate used by Logs.svelte to filter log lines by level + text.
// Extracted so it can be unit-tested without mounting the Svelte component.
export function matchesFilter(l, level, q) {
  return (!level || (l.level || '').toLowerCase() === level) &&
    (!q || (l.text || '').toLowerCase().includes(q.toLowerCase()));
}
