<script lang="ts">
	import { cn, type WithElementRef } from "$lib/utils.js";
	import type { HTMLAttributes } from "svelte/elements";

	let {
		ref = $bindable(null),
		class: className,
		children,
		size = "default",
		interactive = false,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLDivElement>> & {
		size?: "default" | "sm";
		/**
		 * The whole card is a click target: quiet hover affordance (border darkens,
		 * slight lift), pointer cursor, keyboard focus ring. Static containers omit
		 * it — one Card, two modes.
		 */
		interactive?: boolean;
	} = $props();
</script>

<div
	bind:this={ref}
	data-slot="card"
	data-size={size}
	class={cn(
		"bg-card text-card-foreground flex flex-col gap-6 rounded-lg border py-6",
		interactive &&
			"cursor-pointer transition-colors hover:border-foreground/20 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
		className
	)}
	{...restProps}
>
	{@render children?.()}
</div>
