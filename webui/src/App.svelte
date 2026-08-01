<script>
  import Router, { push, location } from 'svelte-spa-router';
  import routes from './lib/routes.js';
  import Nav from './lib/Nav.svelte';
  import { authed } from './lib/auth.js';

  // redirect to /login when not authed (except on the login route)
  $: if (!$authed && $location !== '/login') push('/login');
</script>

{#if $authed}
  <div class="flex min-h-screen">
    <Nav />
    <div class="flex-1 p-6"><Router {routes} /></div>
  </div>
{:else}
  <Router {routes} />
{/if}
