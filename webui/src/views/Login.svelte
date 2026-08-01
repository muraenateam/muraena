<script>
  import { login } from '../lib/auth.js';
  import { push } from 'svelte-spa-router';
  let username = 'admin', password = '', error = '';
  async function submit() {
    error = '';
    try { await login(username, password); push('/'); }
    catch (e) { error = e.message || 'login failed'; }
  }
</script>

<div class="min-h-screen flex items-center justify-center">
  <form on:submit|preventDefault={submit} class="w-80 space-y-4 p-6 rounded-xl bg-slate-900 border border-slate-800">
    <h1 class="text-xl font-semibold">Muraena Console</h1>
    <input class="w-full px-3 py-2 rounded bg-slate-800" bind:value={username} placeholder="username" />
    <input class="w-full px-3 py-2 rounded bg-slate-800" type="password" bind:value={password} placeholder="password" />
    {#if error}<p class="text-red-400 text-sm">{error}</p>{/if}
    <button class="w-full py-2 rounded bg-indigo-600 hover:bg-indigo-500">Sign in</button>
  </form>
</div>
