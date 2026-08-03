<script>
  import { link } from 'svelte-spa-router';
  import { logout } from './auth.js';
</script>
<nav class="w-56 shrink-0 bg-slate-900 border-r border-slate-800 p-4 space-y-1">
  <div class="text-lg font-bold mb-4">Muraena</div>
  <a use:link href="/" class="block px-3 py-2 rounded hover:bg-slate-800">Dashboard</a>
  <a use:link href="/victims" class="block px-3 py-2 rounded hover:bg-slate-800">Victims</a>
  <a use:link href="/traffic" class="block px-3 py-2 rounded hover:bg-slate-800">Traffic</a>
  <a use:link href="/logs" class="block px-3 py-2 rounded hover:bg-slate-800">Logs</a>
  <a use:link href="/config" class="block px-3 py-2 rounded hover:bg-slate-800">Config</a>
  <a use:link href="/recon" class="block px-3 py-2 rounded hover:bg-slate-800">Recon</a>
  <a use:link href="/settings" class="block px-3 py-2 rounded hover:bg-slate-800">Settings</a>
  <button on:click={logout} class="mt-6 w-full text-left px-3 py-2 rounded text-red-400 hover:bg-slate-800">Logout</button>
</nav>
