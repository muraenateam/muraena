<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { connect } from '../lib/ws.js';
  import { lineDiff } from '../lib/diff.js';

  let flows = [], selected = null, live = true, sock = null, filterHost = '';

  async function loadInitial() { flows = await api('/traffic?limit=100'); }
  async function open(id) { selected = await api('/traffic/' + id); }

  function startLive() {
    sock = connect('/ws/traffic', filterHost ? { host: filterHost } : {}, (msg) => {
      if (msg.type === 'flow' && live) flows = [msg.data, ...flows].slice(0, 500);
    });
  }
  function toggleLive() { live = !live; }

  onMount(async () => { await loadInitial(); startLive(); });
  onDestroy(() => sock && sock.close());

  $: diff = selected ? lineDiff(selected.resBodyOriginal, selected.resBodyModified) : [];
</script>

<div class="flex items-center gap-3 mb-4">
  <h1 class="text-2xl font-bold">Traffic</h1>
  <button class="px-2 py-1 rounded bg-slate-700 text-sm" on:click={toggleLive}>{live ? 'Pause' : 'Resume'}</button>
  <input class="px-2 py-1 rounded bg-slate-800 text-sm" placeholder="filter host" bind:value={filterHost} />
  <span class="text-xs text-amber-400">bodies may contain credentials</span>
</div>

<div class="flex gap-4">
  <table class="flex-1 text-xs">
    <thead class="text-slate-400 text-left"><tr><th class="p-1">time</th><th>method</th><th>host</th><th>path</th><th>status</th></tr></thead>
    <tbody>
      {#each flows as f}
        <tr class="border-t border-slate-800 hover:bg-slate-900 cursor-pointer" on:click={() => open(f.id)}>
          <td class="p-1">{f.timestamp?.slice(11,19)}</td>
          <td>{f.method}</td><td>{f.host}</td><td class="truncate max-w-[16rem]">{f.path}</td>
          <td>{f.status}</td>
        </tr>
      {/each}
    </tbody>
  </table>

  {#if selected}
    <div class="w-[40rem] bg-slate-900 border border-slate-800 rounded p-4 overflow-auto max-h-[80vh]">
      <button class="text-slate-400 mb-2" on:click={() => selected = null}>close ✕</button>
      <div class="font-mono text-sm">{selected.method} {selected.host}{selected.path} → {selected.status}</div>
      <h3 class="mt-3 font-semibold text-sm">Response: original vs modified</h3>
      <pre class="text-[11px] leading-tight bg-slate-950 p-2 rounded overflow-auto">{#each diff as d}<span class={d.type === 'add' ? 'text-green-400' : d.type === 'del' ? 'text-red-400' : 'text-slate-300'}>{d.type === 'add' ? '+ ' : d.type === 'del' ? '- ' : '  '}{d.text}
</span>{/each}</pre>
    </div>
  {/if}
</div>
