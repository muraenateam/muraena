import { describe, it, expect } from 'vitest';
import { lineDiff } from './diff.js';

describe('lineDiff', () => {
  it('marks added and deleted lines', () => {
    const d = lineDiff('a\nb\nc', 'a\nB\nc');
    const types = d.map((x) => x.type);
    expect(types).toContain('del'); // b removed
    expect(types).toContain('add'); // B added
    expect(d.filter((x) => x.type === 'same').length).toBe(2); // a, c
  });
});
