<script>
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api.js';

  let cfg = null, restartFields = [], msg = '', err = '';
  // editable live fields (Go-field-name paths)
  let userAgent = '', sameSite = '';
  let importText = '', importFields = [];

  onMount(async () => {
    try {
      cfg = await api('/config');
      restartFields = (await api('/config/restart-fields')).fields ?? [];
      userAgent = cfg?.Transform?.Request?.UserAgent ?? '';
      sameSite = cfg?.Transform?.Response?.Cookie?.SameSite ?? '';
    } catch (e) { err = e.message; }
  });

  async function save() {
    msg = ''; err = '';
    const patch = {
      Transform: {
        Request: { UserAgent: userAgent },
        Response: { Cookie: { SameSite: sameSite } },
      },
    };
    try {
      const r = await api('/config', { method: 'PATCH', body: patch });
      msg = 'applied';
    } catch (e) {
      err = e.message; // 409 restartRequired surfaces here
    }
  }
  async function reload() { await api('/config/reload', { method: 'POST' }); msg = 'reloaded'; }

  async function importFragment() {
    msg = ''; err = ''; importFields = [];
    let patch;
    try {
      patch = JSON.parse(importText);
    } catch (e) {
      err = 'invalid JSON config fragment: ' + e.message;
      return;
    }
    try {
      await api('/config/import', { method: 'POST', body: patch });
      msg = 'imported';
      importText = '';
    } catch (e) {
      if (e instanceof ApiError && e.status === 409 && e.body?.restartRequired) {
        importFields = e.body.fields ?? [];
      } else {
        err = e.message;
      }
    }
  }
</script>

<h1 class="text-2xl font-bold mb-4">Config</h1>
{#if err}<p class="text-red-400">{err}</p>{/if}
{#if msg}<p class="text-green-400">{msg}</p>{/if}

{#if cfg}
  <div class="space-y-4 max-w-xl">
    <label class="block">
      <span class="text-slate-400 text-sm">Request User-Agent (live)</span>
      <input class="w-full px-3 py-2 rounded bg-slate-800" bind:value={userAgent} />
    </label>
    <label class="block">
      <span class="text-slate-400 text-sm">Response Cookie SameSite (live)</span>
      <input class="w-full px-3 py-2 rounded bg-slate-800" bind:value={sameSite} />
    </label>

    <div class="flex gap-2">
      <button class="px-3 py-2 rounded bg-indigo-600" on:click={save}>Apply live</button>
      <button class="px-3 py-2 rounded bg-slate-700" on:click={reload}>Reload from file</button>
    </div>

    <div class="mt-6">
      <h3 class="font-semibold">Restart-required fields</h3>
      <ul class="text-sm text-amber-400 list-disc pl-6">
        {#each restartFields as f}<li>{f} <span class="text-xs text-slate-500">(edit config.toml + restart)</span></li>{/each}
      </ul>
    </div>

    <details class="mt-6">
      <summary class="cursor-pointer text-slate-400">Raw config (read-only, secrets redacted)</summary>
      <pre class="text-xs bg-slate-950 p-2 rounded overflow-auto">{JSON.stringify(cfg, null, 2)}</pre>
    </details>

    <div class="mt-6">
      <h3 class="font-semibold">Import config fragment</h3>
      <p class="text-xs text-slate-500 mb-2">Paste a JSON config patch (e.g. from a recon result) and import it.</p>
      <textarea class="w-full h-32 px-3 py-2 rounded bg-slate-800 font-mono text-xs" bind:value={importText} placeholder={'{ "Origins": { ... } }'}></textarea>
      <button class="mt-2 px-3 py-2 rounded bg-indigo-600" on:click={importFragment}>Import fragment</button>
      {#if importFields.length}
        <div class="mt-2">
          <p class="text-sm text-amber-400">Restart required for:</p>
          <ul class="text-sm text-amber-400 list-disc pl-6">
            {#each importFields as f}<li>{f}</li>{/each}
          </ul>
        </div>
      {/if}
    </div>
  </div>
{/if}
