<script>
	import { goto, invalidateAll } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { login } from '$lib/api.js';
	import { AlertCircle, Github } from '@lucide/svelte';

	let email = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	async function handleSubmit(event) {
		event.preventDefault();
		error = '';
		loading = true;
		try {
			await login({ email, password });
			await invalidateAll();
			await goto('/');
		} catch (err) {
			error = err?.message || 'Unable to login';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Login ~ The Bakery</title>
</svelte:head>

<section class="mx-auto flex min-h-screen w-full max-w-md flex-col justify-center px-6 py-12">
	<header class="mb-8 space-y-2 text-center">
		<h1 class="text-3xl font-semibold tracking-tight">Welcome back</h1>
		<p class="text-sm text-muted-foreground">
			Sign in to manage deployments, domains, databases, and analytics.
		</p>
	</header>

	<form class="space-y-6 rounded-2xl border bg-card p-8 shadow-sm" onsubmit={handleSubmit}>
		<div class="space-y-1.5">
			<label for="email" class="text-sm font-medium">Email</label>
			<input
				bind:value={email}
				id="email"
				type="email"
				class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
				placeholder="you@example.com"
				required
				autocomplete="username"
			/>
		</div>

		<div class="space-y-1.5">
			<label for="password" class="text-sm font-medium">Password</label>
			<input
				bind:value={password}
				id="password"
				type="password"
				class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm transition outline-none focus:border-primary focus:ring-2 focus:ring-primary/20"
				placeholder="••••••••"
				required
				autocomplete="current-password"
			/>
		</div>

		{#if error}
			<div
				class="flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
			>
				<AlertCircle class="mt-0.5 h-4 w-4 " />
				<p>{error}</p>
			</div>
		{/if}

		<Button type="submit" class="w-full justify-center gap-2" disabled={loading}>
			{#if loading}
				<span
					class="h-4 w-4 animate-spin rounded-full border-2 border-primary border-r-transparent"
				></span>
				Signing in…
			{:else}
				Sign in
			{/if}
		</Button>
	</form>
</section>
