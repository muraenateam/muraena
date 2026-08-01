import Login from '../views/Login.svelte';
import Dashboard from '../views/Dashboard.svelte';
import Victims from '../views/Victims.svelte';
import Traffic from '../views/Traffic.svelte';
import Config from '../views/Config.svelte';
import Recon from '../views/Recon.svelte';
import Settings from '../views/Settings.svelte';

export default {
  '/login': Login,
  '/': Dashboard,
  '/victims': Victims,
  '/traffic': Traffic,
  '/config': Config,
  '/recon': Recon,
  '/settings': Settings,
};
