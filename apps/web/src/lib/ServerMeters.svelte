<script lang="ts">
  import { percent, size } from './format'
  import Meter from './Meter.svelte'
  import type { ServerMetrics } from './types'

  let { metrics }: { metrics: ServerMetrics } = $props()
</script>

<div class="meters">
  <Meter label="CPU ({metrics.cpus})" used={metrics.cpu_percent} total={100} text={percent(metrics.cpu_percent)} />
  <Meter
    label="Memory"
    used={metrics.memory_used_bytes}
    total={metrics.memory_total_bytes}
    text="{size(metrics.memory_used_bytes)} / {size(metrics.memory_total_bytes)}"
  />
  <Meter
    label="Disk"
    used={metrics.disk_used_bytes}
    total={metrics.disk_total_bytes}
    text="{size(metrics.disk_used_bytes)} / {size(metrics.disk_total_bytes)}"
  />
</div>

<style>
  .meters {
    display: grid;
    grid-template-columns: repeat(3, minmax(9rem, 1fr));
    gap: 1rem;
  }
</style>
