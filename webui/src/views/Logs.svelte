<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { api } from '../lib/api.js';
  import { connect } from '../lib/ws.js';
  import { matchesFilter } from '../lib/logFilter.js';

  let lines = [], follow = true, level = '', q = '', sock = null, box, err = '';
  // values are the short tokens used by log/level.go LevelNames; labels are for display only
  const LEVELS = [
    { v: '', label: 'all levels' },
    { v: 'ver', label: 'verbose' },
    { v: 'dbg', label: 'debug' },
    { v: 'inf', label: 'info' },
    { v: 'imp', label: 'important' },
    { v: 'war', label: 'warning' },
    { v: 'err', label: 'error' },
    { v: '!!!', label: 'fatal' },
    { v: 'raw', label: 'raw' },
  ];

  const color = (lv) => ({
    err: 'text-red-400', '!!!': 'text-red-500', war: 'text-amber-400',
    imp: 'text-indigo-300', inf: 'text-slate-200', dbg: 'text-slate-400',
    ver: 'text-slate-500', raw: 'text-slate-400',
  }[(lv || '').toLowerCase()] ?? 'text-slate-300');

  $: shown = lines.filter((l) => matchesFilter(l, level, q));

  async function autoscroll() { if (follow) { await tick(); if (box) box.scrollTop = box.scrollHeight; } }

  onMount(async () => {
    try {
      // backfill returns newest-first; reverse to chronological for the console
      const hist = await api('/logs?limit=200');
      lines = hist.slice().reverse();
      await autoscroll();
    } catch (e) { err = e.message; }
    // connect regardless of backfill outcome so live streaming still works
    sock = connect('/ws/logs', {}, (m) => {
      if (m.type === 'log') { lines = [...lines, m.data].slice(-2000); autoscroll(); }
    });
  });
  onDestroy(() => sock && sock.close());
</script>

<div class="flex items-center gap-3 mb-3">
  <h1 class="text-2xl font-bold">Logs</h1>
  <button class="px-2 py-1 rounded bg-slate-700 text-sm" on:click={() => follow = !follow}>{follow ? 'Following' : 'Paused'}</button>
  <select class="px-2 py-1 rounded bg-slate-800 text-sm" bind:value={level}>
    {#each LEVELS as lv}<option value={lv.v}>{lv.label}</option>{/each}
  </select>
  <input class="px-2 py-1 rounded bg-slate-800 text-sm" placeholder="filter text" bind:value={q} />
  <button class="px-2 py-1 rounded bg-slate-700 text-sm" on:click={() => lines = []}>clear</button>
  <span class="text-xs text-amber-400">log lines may contain credentials</span>
</div>

{#if err}<p class="text-red-400 text-sm mb-2">{err}</p>{/if}

<pre bind:this={box} class="text-[11px] leading-tight bg-slate-950 p-3 rounded h-[75vh] overflow-auto font-mono">{#each shown as l}<span class={color(l.level)}>{l.time?.slice(11,19) ?? ''} {(l.level || '').toUpperCase().padEnd(9)} {l.text}
</span>{/each}</pre>
