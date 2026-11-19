<script lang="ts">
  import { copyText } from '../lib/clipboard';
  import Icon from './Icon.svelte';
  import { Button } from '$lib/components/ui/button';
  import { messages, type Locale } from '../lib/i18n';
  import { onDestroy } from 'svelte';

  let { text, label, prominent = false, locale }: { text: string; label?: string; prominent?: boolean; locale: Locale } = $props();
  const wording = $derived(messages[locale]);
  const copyLabel = $derived(label ?? wording.copySource);
  let pending = $state(false);
  let result = $state('');
  let failed = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  function flash(message: string) {
    clearTimeout(timer);
    result = message;
    timer = setTimeout(() => { result = ''; }, 1600);
  }

  $effect(() => {
    text;
    clearTimeout(timer);
    result = '';
    failed = false;
  });

  onDestroy(() => clearTimeout(timer));

  async function copy() {
    const snapshot = text;
    pending = true;
    result = '';
    try {
      await copyText(snapshot);
      if (text === snapshot) {
        failed = false;
        flash(wording.copiedNotice);
      }
    } catch {
      if (text === snapshot) {
        failed = true;
        flash(wording.copyFailed);
      }
    } finally {
      pending = false;
    }
  }
</script>

<div class="copy-control">
  <Button variant={prominent ? 'default' : 'outline'} size="sm" class="copy-icon-button" onclick={copy} disabled={pending || !text} aria-label={copyLabel} title={copyLabel}>
    <Icon name={result && !failed ? 'check' : 'copy'} size={14} />
  </Button>
  <span class="copy-result" class:copy-failed={failed} role="status" aria-live="polite" aria-atomic="true">{result}</span>
</div>
