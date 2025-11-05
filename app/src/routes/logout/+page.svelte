<script>
	import { Button } from "$lib/components/ui/button"
	
	import { goto } from '$app/navigation';
	import { logout as apiLogout } from '$lib/api.js';

	let submitting = $state(false);
	let error = $state(null);

	async function confirmLogout() {
		error = null;
		submitting = true;
		try {
			await apiLogout();
			await goto('/login');
		} catch (err) {
			error = err?.message ?? 'Failed to log out. Please try again.';
			submitting = false;
		}
	}

	function cancelLogout() {
		goto('/');
	}
</script>

<svelte:head>
	<title>Logout ~ The Bakery</title>
</svelte:head>

<section class="flex min-h-[60vh] flex-col items-center justify-center gap-8 p-6 text-center">
	<div class="max-w-md space-y-4">
		<h1 class="text-3xl font-semibold tracking-tight">Confirm logout</h1>
		<p class="text-sm text-muted-foreground">
			You are about to sign out of Bakery. Any unsaved changes in open tabs will be lost. Are you sure you want to continue?
		</p>
	</div>

	{#if error}
		<p class="rounded-md border border-rose-200 bg-rose-50 px-4 py-2 text-sm text-rose-700 dark:border-rose-900/50 dark:bg-rose-900/30 dark:text-rose-200">
			{error}
		</p>
	{/if}

	<div class="flex flex-wrap items-center justify-center gap-3">
		<Button variant="outline" class="min-w-[140px]" disabled={submitting} onclick={cancelLogout}>
			Stay logged in
		</Button>
		<Button class="min-w-[140px]" disabled={submitting} onclick={confirmLogout}>
			{#if submitting}
				Signing out...
			{:else}
				Sign out
			{/if}
		</Button>
	</div>
</section>
