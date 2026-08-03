import { describe, it, expect } from 'vitest';
import { matchesFilter } from './logFilter.js';

describe('matchesFilter', () => {
  // level uses the short tokens from log/level.go LevelNames: ver/dbg/inf/imp/war/err/!!!/raw
  const line = { level: 'war', text: 'Session Hijacked for user@example.com' };

  it('matches level case-insensitively against the line level', () => {
    expect(matchesFilter(line, 'war', '')).toBe(true);
    expect(matchesFilter({ ...line, level: 'WAR' }, 'war', '')).toBe(true);
    expect(matchesFilter(line, 'err', '')).toBe(false);
  });

  it('passes everything when level filter is empty', () => {
    expect(matchesFilter(line, '', '')).toBe(true);
  });

  it('matches text as a case-insensitive substring', () => {
    expect(matchesFilter(line, '', 'hijacked')).toBe(true);
    expect(matchesFilter(line, '', 'HIJACKED')).toBe(true);
    expect(matchesFilter(line, '', 'nope')).toBe(false);
  });

  it('requires both level and text filters to match when both set', () => {
    expect(matchesFilter(line, 'war', 'hijacked')).toBe(true);
    expect(matchesFilter(line, 'err', 'hijacked')).toBe(false);
    expect(matchesFilter(line, 'war', 'nope')).toBe(false);
  });

  it('treats missing level/text fields as empty strings', () => {
    expect(matchesFilter({}, '', 'x')).toBe(false);
    expect(matchesFilter({}, '', '')).toBe(true);
  });

  it('selecting the "war" level keeps only "war" lines out of a mixed set', () => {
    const mixed = [
      { level: 'inf', text: 'connected' },
      { level: 'war', text: 'session hijacked' },
      { level: 'err', text: 'timeout' },
      { level: 'war', text: 'retrying' },
    ];
    const filtered = mixed.filter((l) => matchesFilter(l, 'war', ''));
    expect(filtered).toEqual([
      { level: 'war', text: 'session hijacked' },
      { level: 'war', text: 'retrying' },
    ]);
  });
});
