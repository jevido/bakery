<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> = $props();
</script>

<div
	bind:this={ref}
	data-slot="card-header"
	class={cn(
		"@container/card-header grid auto-rows-min grid-rows-(--gtr-1) items-start gap-2 px-6 has-data-[slot=card-action]:grid-cols-(--gtc-17) [.border-b]:pb-6",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</div>
