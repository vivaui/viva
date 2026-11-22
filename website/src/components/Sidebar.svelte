<script lang="ts">
  import { onMount } from 'svelte';
  import type { Nav } from '../lib/types';
  import { filterNavigation } from '../lib/preview';
  import Icon from './Icon.svelte';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { messages, componentPath, categoryLabel, componentLabel, type Locale } from '../lib/i18n';

  let { items, currentID, locale }: { items: Nav[]; currentID: string; locale: Locale } = $props();
  const text = $derived(messages[locale]);
  let query = $state('');
  let open = $state(false);
  let mounted = $state(false);
  let search = $state<HTMLInputElement | null>(null);
  let toggle = $state<HTMLButtonElement | HTMLAnchorElement | null>(null);
  let navigation: HTMLElement;
  let panel: HTMLDivElement;
  const scrollKey = $derived(`viva-docs-sidebar-scroll:${locale}`);
  const current = $derived(items.find(item => item.ID === currentID));
  const filtered = $derived(filterNavigation(items.map(item => ({ ...item, Title: componentLabel(locale, item.ID, item.Title), Category: categoryLabel(locale, item.Category) })), query));


  onMount(() => {
    mounted = true;
    try {
      const saved = Number(sessionStorage.getItem(scrollKey));
      if (Number.isFinite(saved) && saved > 0) panel.scrollTop = saved;
    } catch {}
  });

  function rememberScroll() {
    try {
      sessionStorage.setItem(scrollKey, String(panel.scrollTop));
    } catch {}
  }

  function focusSearch() {
    open = true;
    requestAnimationFrame(() => { search?.focus(); search?.select(); });
  }

  function shortcut(event: KeyboardEvent) {
    if (event.isComposing || event.defaultPrevented) return;
    const target = event.target;
    const editing = target instanceof HTMLElement && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName));
    // The site-wide dialog owns Cmd/Ctrl+K; this field keeps the slash shortcut.
    if (event.key === '/' && !editing && !event.metaKey && !event.ctrlKey && !event.altKey) {
      event.preventDefault();
      focusSearch();
    }
  }

  function searchKey(event: KeyboardEvent) {
    if (event.isComposing) return;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      navigation.querySelector<HTMLAnchorElement>('a')?.focus();
    }
  }

  function navigationKey(event: KeyboardEvent) {
    const links = [...navigation.querySelectorAll<HTMLAnchorElement>('a')];
    const current = links.indexOf(document.activeElement as HTMLAnchorElement);
    if (current < 0) return;
    let next = current;
    if (event.key === 'ArrowDown') next = Math.min(current + 1, links.length - 1);
    else if (event.key === 'ArrowUp') next = Math.max(current - 1, 0);
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = links.length - 1;
    else return;
    event.preventDefault();
    links[next]?.focus();
  }

  function escape(event: KeyboardEvent) {
    if (event.key !== 'Escape' || event.isComposing) return;
    if (query) {
      query = '';
      search?.focus();
    } else if (open && matchMedia('(max-width: 800px)').matches) {
      open = false;
      toggle?.focus();
    }
  }
</script>

<svelte:window onkeydown={shortcut} />

<div class="sidebar-shell" class:enhanced={mounted}>
  <Button variant="ghost" class="mobile-nav-toggle hidden max-[800px]:flex" bind:ref={toggle} onclick={() => { open = !open; }} onkeydown={escape} aria-expanded={open} aria-controls="sidebar-panel" disabled={!mounted}>
    <span><Icon name={open ? 'close' : 'menu'} /> {text.browse}</span>
    <span class="muted">{current ? componentLabel(locale, current.ID, current.Title) : text.components}</span>
  </Button>
  <div class="sidebar-panel" id="sidebar-panel" class:open bind:this={panel}>
    <div class="sidebar-top">
      <div class="search-box">
        <Icon name="search" />
        <label for="component-search" class="sr-only">{text.search}</label>
        <Input id="component-search" type="search" placeholder={text.searchPlaceholder} bind:value={query} bind:ref={search} onkeydown={(event) => { escape(event); searchKey(event); }} aria-controls="component-navigation" autocomplete="off" disabled={!mounted} class="pl-8 pr-8" />
        <kbd aria-hidden="true">/</kbd>
      </div>
      <div class="nav-heading"><span>{text.components}</span></div>
      <nav id="component-navigation" aria-label={text.components} bind:this={navigation}>
        <ul class="component-list">
          {#each filtered as item (item.ID)}
            <li><a href={componentPath(locale, item.ID)} class="nav-link" onclick={rememberScroll} onkeydown={(event) => { escape(event); navigationKey(event); }} aria-current={currentID === item.ID ? 'page' : undefined}><span>{item.Title}</span>{#if currentID === item.ID}<span class="nav-active-mark" aria-hidden="true"></span>{/if}</a></li>
          {/each}
        </ul>
      </nav>
      {#if !filtered.length}<p class="search-empty">{text.noMatches}</p>{/if}
      <p class="sr-only" role="status" aria-live="polite">{query ? `${filtered.length} ${text.matches}` : ''}</p>
    </div>
  </div>
</div>
