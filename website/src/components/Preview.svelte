<script lang="ts">
  import { onMount } from 'svelte';
  import type { Demo, PreviewState, Theme } from '../lib/types';
  import { messages, type Locale } from '../lib/i18n';
  import { previewMeasurement, previewPath, previewTimeout, previewUpdate, previewWheel } from '../lib/preview';
  import { Button } from '$lib/components/ui/button';
  import Icon from './Icon.svelte';

  let { componentID, title, example, revision, locale, height }: {
    componentID: string; title: string; example: Demo; revision: string; locale: Locale; height?: number;
  } = $props();
  let measuredHeight = $state<number | null>(null);
  const canvasHeight = $derived(measuredHeight ?? (height && height > 0 ? height : 260));
  const text = $derived(messages[locale]);
  let theme = $state<Theme>('light');
  let mounted = $state(false);
  let activated = $state(false);
  let resetVersion = $state(0);
  let loadState = $state<PreviewState>('loading');
  let message = $state('');
  let container = $state<HTMLDivElement>();
  let frameMount = $state<HTMLDivElement>();
  let activeFrame: HTMLIFrameElement | null = null;
  let timeout: ReturnType<typeof setTimeout> | undefined;
  const path = $derived(previewPath(componentID, example.ID, theme, revision, locale));

  onMount(() => {
    theme = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
    mounted = true;
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        activated = true;
        observer.disconnect();
      }
    }, { rootMargin: '200px' });
    if (container) observer.observe(container);
    const receiveTheme = (event: Event) => {
      const next = (event as CustomEvent<{ theme: Theme }>).detail?.theme;
      if (next === 'dark' || next === 'light') theme = next;
    };
    const receiveMessage = (event: MessageEvent) => {
      const measured = previewMeasurement(event, window.location.origin, activeFrame?.contentWindow ?? null);
      if (measured !== null) {
        measuredHeight = measured;
        return;
      }
      const wheel = previewWheel(event, window.location.origin, activeFrame?.contentWindow ?? null);
      if (wheel) {
        window.scrollBy(wheel.deltaX, wheel.deltaY);
        return;
      }
      const update = previewUpdate(event, window.location.origin, activeFrame?.contentWindow ?? null, loadState);
      if (!update) return;
      clearTimeout(timeout);
      loadState = update.state;
      message = update.message;
    };
    window.addEventListener('viva-theme-change', receiveTheme);
    window.addEventListener('message', receiveMessage);
    return () => {
      observer.disconnect();
      window.removeEventListener('viva-theme-change', receiveTheme);
      window.removeEventListener('message', receiveMessage);
      clearTimeout(timeout);
    };
  });

  $effect(() => {
    const source = path;
    resetVersion;
    if (!mounted || !activated || !frameMount) return;
    clearTimeout(timeout);
    loadState = 'loading';
    measuredHeight = null;
    message = '';
    const frame = document.createElement('iframe');
    frame.title = `${title}: ${example.Title} — Go WebAssembly`;
    frame.src = source;
    frame.className = 'preview-frame';
    frame.referrerPolicy = 'same-origin';
    activeFrame = frame;
    frameMount.replaceChildren(frame);
    timeout = setTimeout(() => {
      if (activeFrame !== frame) return;
      loadState = 'error';
      message = '';
    }, previewTimeout);
    return () => {
      clearTimeout(timeout);
      if (activeFrame === frame) activeFrame = null;
      frame.remove();
    };
  });

  function reset() {
    activated = true;
    resetVersion += 1;
  }

</script>

<div class="example-demo" bind:this={container}>
  <div class="example-toolbar">
    <div class="window-dots" aria-hidden="true"><span class="window-dot window-dot-close"></span><span class="window-dot window-dot-min"></span><span class="window-dot window-dot-max"></span></div>
    <span class="window-title">{example.Title}</span>
    <div class="window-actions">
      <button type="button" class="window-action" onclick={reset} disabled={!mounted} aria-label={`${text.resetExample}: ${example.Title}`} title={text.reset}><Icon name="reset" size={14} /></button>
      <a class="window-action" href={path} target="_blank" rel="noopener noreferrer" aria-label={`${text.openPreview}: ${example.Title}`} title={text.openPreview}><Icon name="arrow" size={14} /></a>
    </div>
  </div>
  <div class="preview-canvas" style={`height:${canvasHeight}px`} data-state={loadState} aria-label={`${example.Title} ${text.preview}`} aria-busy={mounted && activated && loadState === 'loading'}>
    <div class="preview-mount" bind:this={frameMount}></div>
    {#if loadState === 'loading'}
      <div class="preview-overlay"><p>{!mounted ? text.noJS : activated ? text.starting : text.waiting}</p></div>
    {:else if loadState === 'error'}
      <div class="preview-overlay preview-error"><Icon name="warning" size={24} /><strong>{text.unavailable}</strong><p>{message ? `${text.runtimeError} ${message}` : text.timeout}</p><Button variant="outline" onclick={reset}>{text.retry}</Button></div>
    {/if}
  </div>
</div>
