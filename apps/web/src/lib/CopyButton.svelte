<script lang="ts">
  let { text, label = 'Copy' }: { text: string; label?: string } = $props()

  let copied = $state(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  async function copy() {
    await navigator.clipboard.writeText(text)
    copied = true
    clearTimeout(timer)
    timer = setTimeout(() => (copied = false), 1500)
  }
</script>

<button type="button" onclick={copy}>{copied ? 'Copied' : label}</button>
