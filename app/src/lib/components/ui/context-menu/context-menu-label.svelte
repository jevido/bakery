<script>
	import { cn } from '$lib/utils.js';
	let { ref = $bindable(null), class: className, inset, children, ...restProps } = $props();
</script>

<div
	bind:this={ref}
	data-slot="context-menu-label"
	data-inset={inset}
	class={cn('px-2 py-1.5 text-sm font-medium text-foreground data-[inset]:pl-8', className)}
	{...restProps}
>
	{@render children?.()}
</div>
