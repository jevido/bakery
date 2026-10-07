<script lang="ts">
	import { Avatar as AvatarPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		loadingStatus = $bindable("loading"),
		size = "default",
		shape = "circle",
		class: className,
		...restProps
	}: AvatarPrimitive.RootProps & {
		size?: "default" | "xs" | "sm" | "lg";
		shape?: "circle" | "square";
	} = $props();
</script>

<AvatarPrimitive.Root
	bind:ref
	bind:loadingStatus
	data-slot="avatar"
	data-size={size}
	data-shape={shape}
	class={cn(
		"group/avatar relative flex size-8 shrink-0 overflow-hidden select-none data-[size=lg]:size-10 data-[size=sm]:size-6 data-[size=xs]:size-5",
		shape === "square" ? "rounded-md" : "rounded-full",
		className
	)}
	{...restProps}
/>
