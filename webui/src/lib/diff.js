// Minimal LCS-free line diff: good enough for body comparison display.
export function lineDiff(a, b) {
  const A = (a ?? '').split('\n');
  const B = (b ?? '').split('\n');
  const out = [];
  let i = 0, j = 0;
  while (i < A.length || j < B.length) {
    if (i < A.length && j < B.length && A[i] === B[j]) {
      out.push({ type: 'same', text: A[i] }); i++; j++;
    } else if (j < B.length && (i >= A.length || A[i] !== B[j]) && !A.includes(B[j])) {
      out.push({ type: 'add', text: B[j] }); j++;
    } else if (i < A.length && !B.includes(A[i])) {
      out.push({ type: 'del', text: A[i] }); i++;
    } else {
      // fallback: advance both
      if (i < A.length) out.push({ type: 'del', text: A[i++] });
      if (j < B.length) out.push({ type: 'add', text: B[j++] });
    }
  }
  return out;
}
