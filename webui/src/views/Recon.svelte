<script>
  import { api, ApiError } from '../lib/api.js';
  import { connect } from '../lib/ws.js';

  let target = '', depth = 1, maxPages = 20, apply = true;
  let log = [], reconID = '', status = '', result = null, err = '', sock = null;
  let applyRestartFields = [];

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

  async function applyResult() {
    err = ''; applyRestartFields = [];
    try {
      await api('/recon/' + reconID + '/apply', { method: 'POST' });
      status = 'applied';
    } catch (e) {
      if (e instanceof ApiError && e.status === 409 && e.body?.restartRequired) {
        applyRestartFields = e.body.fields ?? [];
      } else {
        err = e.message;
      }
    }
  }
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

  <div class="mt-2">
    <h4 class="font-semibold text-sm text-slate-300">Origins ({result.origins?.length ?? 0})</h4>
    <ul class="text-xs font-mono list-disc pl-6">
      {#each result.origins ?? [] as o}<li>{o}</li>{/each}
    </ul>
  </div>

  <div class="mt-3">
    <h4 class="font-semibold text-sm text-slate-300">Login pages ({result.loginPages?.length ?? 0})</h4>
    <table class="text-xs w-full">
      <thead class="text-slate-400 text-left"><tr><th class="p-1">URL</th><th>Action</th><th>User field</th><th>Pass field</th></tr></thead>
      <tbody>
        {#each result.loginPages ?? [] as p}
          <tr class="border-t border-slate-800">
            <td class="p-1 font-mono">{p.url}</td>
            <td class="font-mono">{p.action}</td>
            <td class="font-mono">{p.usernameSelector}</td>
            <td class="font-mono">{p.passwordSelector}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <div class="mt-3">
    <h4 class="font-semibold text-sm text-slate-300">Secrets paths ({result.secretsPaths?.length ?? 0})</h4>
    <ul class="text-xs font-mono list-disc pl-6">
      {#each result.secretsPaths ?? [] as p}<li>{p}</li>{/each}
    </ul>
  </div>

  <div class="mt-3">
    <h4 class="font-semibold text-sm text-slate-300">Secrets patterns ({result.secretsPatterns?.length ?? 0})</h4>
    <table class="text-xs w-full">
      <thead class="text-slate-400 text-left"><tr><th class="p-1">Label</th><th>Matching</th><th>Start</th><th>End</th></tr></thead>
      <tbody>
        {#each result.secretsPatterns ?? [] as sp}
          <tr class="border-t border-slate-800">
            <td class="p-1 font-mono">{sp.label}</td>
            <td class="font-mono">{sp.matching}</td>
            <td class="font-mono">{sp.start}</td>
            <td class="font-mono">{sp.end}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  {#if !apply}<button class="mt-3 px-3 py-2 rounded bg-green-600" on:click={applyResult}>Apply to config</button>{/if}

  {#if applyRestartFields.length}
    <div class="mt-2">
      <p class="text-sm text-amber-400">Restart required for:</p>
      <ul class="text-sm text-amber-400 list-disc pl-6">
        {#each applyRestartFields as f}<li>{f}</li>{/each}
      </ul>
    </div>
  {/if}
{/if}
