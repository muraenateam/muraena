<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';

  let settings = {}, users = [], newUser = { username: '', password: '', role: 'admin' }, msg = '', err = '';

  async function load() {
    settings = await api('/settings');
    users = await api('/users');
  }
  async function saveSettings() {
    try { settings = await api('/settings', { method: 'PUT', body: settings }); msg = 'saved'; }
    catch (e) { err = e.message; }
  }
  async function addUser() {
    try { await api('/users', { method: 'POST', body: newUser }); newUser = { username: '', password: '', role: 'admin' }; await load(); }
    catch (e) { err = e.message; }
  }
  async function delUser(name) {
    if (!confirm('Delete user ' + name + '?')) return;
    try { await api('/users/' + name, { method: 'DELETE' }); await load(); }
    catch (e) { err = e.message; } // last-admin guard -> 409 message
  }
  onMount(load);
</script>

<h1 class="text-2xl font-bold mb-4">Settings</h1>
{#if err}<p class="text-red-400">{err}</p>{/if}
{#if msg}<p class="text-green-400">{msg}</p>{/if}

<section class="mb-8 max-w-md">
  <h3 class="font-semibold mb-2">JWT settings</h3>
  {#each Object.keys(settings) as k}
    <label class="block mb-2"><span class="text-slate-400 text-sm">{k}</span>
      <input class="w-full px-3 py-2 rounded bg-slate-800" bind:value={settings[k]} /></label>
  {/each}
  <button class="px-3 py-2 rounded bg-indigo-600" on:click={saveSettings}>Save</button>
</section>

<section class="max-w-lg">
  <h3 class="font-semibold mb-2">Users</h3>
  <table class="w-full text-sm mb-3">
    <tbody>
      {#each users as u}
        <tr class="border-t border-slate-800">
          <td class="p-2 font-mono">{u.username}</td><td>{u.role}</td>
          <td><button class="text-red-400" on:click={() => delUser(u.username)}>delete</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
  <div class="flex gap-2">
    <input class="px-2 py-1 rounded bg-slate-800" placeholder="username" bind:value={newUser.username} />
    <input class="px-2 py-1 rounded bg-slate-800" placeholder="password" type="password" bind:value={newUser.password} />
    <button class="px-3 py-1 rounded bg-slate-700" on:click={addUser}>Add</button>
  </div>
</section>
