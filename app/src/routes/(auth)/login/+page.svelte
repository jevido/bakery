<script>
	import { goto, invalidateAll } from '$app/navigation';
	import { login } from '$lib/api.js';
	import { Button } from 'bits-ui';
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

	async function proceedToGithub() {
		const response = await fetch('/api/auth/github/url', {
			credentials: 'include'
		});
		if (response.ok) {
			const { url } = await response.json();
			if (url) {
				window.location.href = url;
				return;
			}
		}
		error = 'Sign in with your Bakery credentials before linking GitHub.';
	}
</script>

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
				id="email"
				type="email"
				class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/20"
				placeholder="you@example.com"
				bind:value={email}
				required
				autocomplete="username"
			/>
		</div>

		<div class="space-y-1.5">
			<label for="password" class="text-sm font-medium">Password</label>
			<input
				id="password"
				type="password"
				class="h-11 w-full rounded-lg border border-input bg-background px-3 text-sm outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/20"
				placeholder="••••••••"
				bind:value={password}
				required
				autocomplete="current-password"
			/>
		</div>

		{#if error}
			<div class="flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
				<AlertCircle class="mt-0.5 h-4 w-4 flex-shrink-0" />
				<p>{error}</p>
			</div>
		{/if}

		<Button.Root class="w-full justify-center gap-2" disabled={loading}>
			{#if loading}
				<span class="h-4 w-4 animate-spin rounded-full border-2 border-primary border-r-transparent"></span>
				Signing in…
			{:else}
				Sign in
			{/if}
		</Button.Root>
	</form>

	<div class="mt-6 space-y-3 text-center text-sm text-muted-foreground">
		<div class="relative h-px w-full bg-border">
			<span class="absolute inset-x-0 -top-3 mx-auto w-fit bg-background px-3 text-xs uppercase tracking-wide text-muted-foreground">
				or
			</span>
		</div>
		<Button.Root variant="outline" class="w-full justify-center gap-2" onclick={proceedToGithub}>
			<Github class="h-4 w-4" />
			Continue with GitHub
		</Button.Root>
	</div>
</section>
