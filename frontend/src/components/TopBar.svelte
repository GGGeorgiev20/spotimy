<script lang="ts">
  import Icon from './Icon.svelte'
  import { back, forward, canBack, canForward, navigate, current } from '../lib/router.svelte'
  import { me } from '../lib/me.svelte'
  import { search, setQuery } from '../lib/search.svelte'
  import { pick, App } from '../lib/api.svelte'

  let menu = $state(false)
  const home = $derived(current().path === '/')
  const initial = $derived(me.user?.name?.[0]?.toUpperCase() ?? '')
</script>

<div class="top">
  <div class="nav">
    <button class="circle" disabled={!canBack()} onclick={back} aria-label="Go back"><Icon name="chevron-left" /></button>
    <button class="circle" disabled={!canForward()} onclick={forward} aria-label="Go forward"><Icon name="chevron-right" /></button>
  </div>

  <div class="center">
    <button class="home" class:on={home} onclick={() => navigate('/')} aria-label="Home">
      <Icon name={home ? 'home-filled' : 'home'} size={24} />
    </button>
    <div class="search">
      <Icon name="search" size={24} />
      <input
        placeholder="What do you want to play?"
        value={search.q}
        oninput={(e) => setQuery(e.currentTarget.value)}
        onfocus={() => !current().path.startsWith('/search') && navigate(search.q.trim() ? `/search/${encodeURIComponent(search.q.trim())}` : '/search')}
        onkeydown={(e) => e.key === 'Escape' && e.currentTarget.blur()}
      />
      {#if search.q}<button class="clear" aria-label="Clear search" onclick={() => setQuery('')}><Icon name="close" /></button>{/if}
    </div>
  </div>

  <div class="right">
    <button class="avatar" onclick={() => (menu = !menu)} aria-label="Profile" style:background={me.user?.color}>
      {#if me.user?.images?.length}<img src={pick(me.user.images, 32)} alt="" />{:else}{initial}{/if}
    </button>
    {#if menu}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="backdrop" onclick={() => (menu = false)}></div>
      <div class="menu" role="menu">
        <button class="mi" onclick={() => ((menu = false), App.OpenAccountPage())}>Account <Icon name="external" /></button>
        <button class="mi" onclick={() => ((menu = false), navigate(`/user/${me.user?.username}`))}>Profile</button>
        <hr />
        <button class="mi" onclick={() => ((menu = false), App.Logout())}>Log out</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .top { height: 64px; display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; padding: 0 8px 0 16px; --wails-draggable: drag; }
  .nav { display: flex; gap: 8px; }
  .circle { width: 32px; height: 32px; border-radius: 50%; background: rgba(0,0,0,.7); display: grid; place-items: center; color: var(--text); --wails-draggable: no-drag; }
  .circle:disabled { opacity: .6; cursor: not-allowed; color: var(--text-2); }
  .center { display: flex; align-items: center; gap: 8px; --wails-draggable: no-drag; }
  .home { width: 48px; height: 48px; border-radius: 50%; background: var(--surface-2); display: grid; place-items: center; color: var(--text-2); }
  .home:hover { transform: scale(1.04); color: var(--text); }
  .home.on { color: var(--text); }
  .search { width: clamp(280px, 30vw, 474px); height: 48px; display: flex; align-items: center; gap: 12px; padding: 0 16px 0 12px; border-radius: 999px; background: var(--surface-2); color: var(--text-2); border: 1px solid transparent; }
  .search:hover { background: var(--surface-3); border-color: hsla(0,0%,100%,.1); }
  .search:focus-within { border-color: var(--text); color: var(--text); }
  .search input { flex: 1; background: none; border: 0; outline: 0; font-size: 16px; }
  .clear { color: var(--text-2); display: grid; }
  .clear:hover { color: var(--text); }
  .search input::placeholder { color: var(--text-2); }
  .right { display: flex; justify-content: flex-end; position: relative; --wails-draggable: no-drag; }
  .avatar { width: 48px; height: 48px; border-radius: 50%; display: grid; place-items: center; background: var(--surface-2); padding: 8px; }
  .avatar:hover { transform: scale(1.04); }
  .avatar img, .avatar { overflow: hidden; }
  .avatar img { width: 32px; height: 32px; border-radius: 50%; }
  .avatar { font-weight: 700; color: #000; }
  .backdrop { position: fixed; inset: 0; z-index: 50; }
  .menu { position: absolute; top: 56px; right: 0; z-index: 51; width: 196px; background: var(--surface-3); border-radius: 4px; padding: 4px; box-shadow: 0 16px 24px rgba(0,0,0,.3), 0 6px 8px rgba(0,0,0,.2); }
  .mi { width: 100%; display: flex; justify-content: space-between; align-items: center; padding: 12px 8px 12px 12px; font-size: 14px; color: hsla(0,0%,100%,.9); border-radius: 2px; }
  .mi:hover { background: hsla(0,0%,100%,.1); }
  hr { border: 0; border-top: 1px solid hsla(0,0%,100%,.1); margin: 0; }
</style>
