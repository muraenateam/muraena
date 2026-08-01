<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  let victims = 0, hijacked = 0, instrumented = 0, keepalives = 0, flows = 0, err = '';

  onMount(async () => {
    try {
      victims = (await api('/victims')).length;
      hijacked = (await api('/sessions/hijacked')).length;
      instrumented = (await api('/sessions/instrumented')).length;
      keepalives = (await api('/keepalives')).length;
      flows = (await api('/traffic/stats')).count ?? 0;
    } catch (e) { err = e.message; }
  });

  const tiles = () => [
    { label: 'Victims', value: victims },
    { label: 'Hijacked', value: hijacked },
    { label: 'Instrumented', value: instrumented },
    { label: 'Keepalives', value: keepalives },
    { label: 'Flows', value: flows },
  ];
</script>

<h1 class="text-2xl font-bold mb-6">Dashboard</h1>
{#if err}<p class="text-red-400">{err}</p>{/if}
<div class="grid grid-cols-2 md:grid-cols-5 gap-4">
  {#each tiles() as t}
    <div class="p-4 rounded-xl bg-slate-900 border border-slate-800">
      <div class="text-3xl font-bold">{t.value}</div>
      <div class="text-slate-400 text-sm">{t.label}</div>
    </div>
  {/each}
</div>
