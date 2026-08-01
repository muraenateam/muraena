<script>
  import { api } from '../lib/api.js';
  import { connect } from '../lib/ws.js';

  let target = '', depth = 1, maxPages = 20, apply = true;
  let log = [], reconID = '', status = '', result = null, err = '', sock = null;

  async function start() {
    err = ''; log = []; result = null; status = 'running';
    try {
      const r = await api('/recon', { method: 'POST', body: { target, depth, maxPages, apply } });
      reconID = r.reconID;
      sock = connect('/ws/recon', { id: reconID }, (m) => { if (m.type === 'progress') log = [...log, m.msg]; });
      poll();
    } catch (e) { err = e.message; status = 'failed'; }
  }

  async function poll() {
    const j = await api('/recon/' + reconID);
    status = j.status;
    if (j.status === 'running') { setTimeout(poll, 800); return; }
    result = j.result;
    if (sock) sock.close();
  }

  async function applyResult() { await api('/recon/' + reconID + '/apply', { method: 'POST' }); status = 'applied'; }
</script>

<h1 class="text-2xl font-bold mb-4">Recon</h1>
{#if err}<p class="text-red-400">{err}</p>{/if}

<div class="flex flex-wrap gap-2 items-end max-w-2xl">
  <label class="flex-1"><span class="text-slate-400 text-sm">Target URL</span>
    <input class="w-full px-3 py-2 rounded bg-slate-800" bind:value={target} placeholder="https://target.tld" /></label>
  <label><span class="text-slate-400 text-sm">Depth</span>
    <input type="number" class="w-20 px-2 py-2 rounded bg-slate-800" bind:value={depth} /></label>
  <label><span class="text-slate-400 text-sm">Max pages</span>
    <input type="number" class="w-24 px-2 py-2 rounded bg-slate-800" bind:value={maxPages} /></label>
  <label class="flex items-center gap-1 text-sm"><input type="checkbox" bind:checked={apply} /> auto-apply</label>
  <button class="px-3 py-2 rounded bg-indigo-600" on:click={start}>Run</button>
</div>

<p class="mt-4 text-sm text-slate-400">Status: {status}</p>
<pre class="mt-2 text-xs bg-slate-950 p-2 rounded h-48 overflow-auto">{log.join('\n')}</pre>

{#if result}
  <h3 class="mt-4 font-semibold">Result</h3>
  <pre class="text-xs bg-slate-950 p-2 rounded overflow-auto">{JSON.stringify(result, null, 2)}</pre>
  {#if !apply}<button class="mt-2 px-3 py-2 rounded bg-green-600" on:click={applyResult}>Apply to config</button>{/if}
{/if}
