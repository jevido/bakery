<script lang="ts">
  let {
    label,
    value = $bindable(''),
    type = 'text',
    error = '',
    autocomplete,
    placeholder,
    required = false,
  }: {
    label: string
    value?: string
    type?: string
    error?: string
    autocomplete?: HTMLInputElement['autocomplete']
    placeholder?: string
    required?: boolean
  } = $props()
</script>

<label class="field">
  <span>{label}</span>
  <input {type} bind:value {autocomplete} {placeholder} {required} aria-invalid={error ? 'true' : undefined} />
  {#if error}<small class="error">{error}</small>{/if}
</label>

<style>
  .field {
    display: grid;
    gap: 0.3rem;
  }
  span {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .error {
    color: var(--danger);
  }
</style>
