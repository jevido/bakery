<script lang="ts">
	import { Switch as SwitchPrimitive } from "bits-ui";
	import { cn, type WithoutChildrenOrChild } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		checked = $bindable(false),
		size = "default",
		...restProps
	}: WithoutChildrenOrChild<SwitchPrimitive.RootProps> & {
		size?: "default" | "lg";
	} = $props();

	const isLg = $derived(size === "lg");
</script>

<SwitchPrimitive.Root
	bind:ref
	bind:checked
	data-slot="switch"
	data-size={size}
	class={cn(
		"relative inline-flex shrink-0 items-center rounded-full border-2 transition-all outline-none",
		"focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30",
		"disabled:cursor-not-allowed disabled:opacity-50",
		isLg ? "h-6 w-12" : "h-5 w-11",
		checked
			? "border-(--status-task-done) bg-(--status-task-done)"
			: "border-transparent bg-input/90",
		className
	)}
	{...restProps}
>
	<SwitchPrimitive.Thumb
		data-slot="switch-thumb"
		class={cn(
			"pointer-events-none inline-block rounded-full bg-background shadow-sm transition-transform not-dark:bg-clip-padding dark:bg-foreground",
			isLg ? "h-5 w-7" : "h-4 w-6",
			checked ? "translate-x-4" : "translate-x-0"
		)}
	/>
</SwitchPrimitive.Root>
