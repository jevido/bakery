<script lang="ts">
  let { error }: { error?: string } = $props()
</script>

{#if error}
  <p class="mt-1 text-xs text-red-500">{error}</p>
{/if}
