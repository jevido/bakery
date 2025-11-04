<script>
	import DotIcon from '@lucide/svelte/icons/dot';

	let { ref = $bindable(null), children, ...restProps } = $props();
</script>

<div bind:this={ref} data-slot="input-otp-separator" role="separator" {...restProps}>
	{#if children}
		{@render children?.()}
	{:else}
		<DotIcon />
	{/if}
</div>
