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
  <!-- A number input binds a number (or null); value stays a string. -->
  <input {type} bind:value={() => value, (v) => (value = v == null ? '' : String(v))} {autocomplete} {placeholder} {required} aria-invalid={error ? 'true' : undefined} />
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
