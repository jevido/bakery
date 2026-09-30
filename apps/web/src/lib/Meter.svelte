<script lang="ts">
  let { label, used, total, text }: { label: string; used: number; total: number; text: string } = $props()

  let share = $derived(total > 0 ? Math.min(100, (used / total) * 100) : 0)
</script>

<div class="meter">
  <div class="line">
    <span class="muted">{label}</span>
    <span>{text}</span>
  </div>
  <div class="bar" role="meter" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(share)}>
    <div class={['fill', share >= 90 && 'high']} style:width="{share}%"></div>
  </div>
</div>

<style>
  .meter {
    display: grid;
    gap: 0.25rem;
    min-width: 9rem;
  }
  .line {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    font-size: 0.8rem;
  }
  .bar {
    height: 6px;
    border-radius: 3px;
    background: var(--border);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--ok);
  }
  .fill.high {
    background: var(--danger);
  }
</style>
