<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';

  let cfg = null, restartFields = [], msg = '', err = '';
  // editable live fields (Go-field-name paths)
  let userAgent = '', sameSite = '';

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
  </div>
{/if}
