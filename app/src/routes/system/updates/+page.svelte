<script>
	import { Button } from '$lib/components/ui/button';
	import { AlertTriangle, CheckCircle2, Loader2, RefreshCw } from '@lucide/svelte';

	let { data } = $props();
	let updateInfo = $state(data.selfUpdate ?? { status: { status: 'idle' }, config: {} });
	let refreshing = $state(false);
	let refreshError = $state(null);

	let status = $derived(() => updateInfo.status ?? { status: 'idle' });
	let config = $derived(() => updateInfo.config ?? {});
	let isRunning = $derived(() => status.status === 'running');
	let lastResult = $derived(() => status.lastResult ?? null);
	let triggerMeta = $derived(() => status.meta ?? (lastResult?.meta ?? null));

	function formatTime(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return date.toLocaleString();
	}

	async function refreshStatus() {
		refreshError = null;
		refreshing = true;

		try {
			const response = await fetch('/api/system/self-update', {
				credentials: 'include'
			});
			if (!response.ok) {
				throw new Error('Unable to load self-update status');
			}
			updateInfo = await response.json();
		} catch (error) {
			refreshError = error?.message ?? 'Failed to refresh status';
		} finally {
			refreshing = false;
		}
	}
</script>

<svelte:head>
	<title>Self-update ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Self-update automation</h1>
			<p class="text-sm text-muted-foreground">
				Track the configured GitHub webhook and the background updater that keeps Bakery
				current.
			</p>
		</div>
		<Button class="gap-2" variant="outline" onclick={refreshStatus} disabled={refreshing}>
			{#if refreshing}
				<Loader2 class="h-4 w-4 animate-spin" />
				Refreshing…
			{:else}
				<RefreshCw class="h-4 w-4" />
				Refresh status
			{/if}
		</Button>
	</header>

	{#if refreshError}
		<div
			class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/30 dark:bg-rose-900/30 dark:text-rose-200"
		>
			{refreshError}
		</div>
	{/if}

	<div class="grid gap-4 md:grid-cols-3">
		<article class="space-y-5 rounded-xl border bg-card p-6 shadow-sm">
			<div class="flex items-center gap-4">
				<div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-secondary">
					{#if isRunning}
						<Loader2 class="h-6 w-6 animate-spin text-secondary-foreground" />
					{:else if lastResult?.success}
						<CheckCircle2 class="h-6 w-6 text-emerald-500" />
					{:else if lastResult}
						<AlertTriangle class="h-6 w-6 text-rose-500" />
					{:else}
						<RefreshCw class="h-6 w-6 text-muted-foreground" />
					{/if}
				</div>
				<div>
					<p class="text-sm font-semibold">Self-update runner</p>
					<p class="text-xs text-muted-foreground">Triggered by GitHub pushes</p>
				</div>
			</div>
			<div class="space-y-1">
				{#if isRunning}
					<p class="text-2xl font-semibold">Updating</p>
					<p class="text-sm text-muted-foreground">Started {formatTime(status.startedAt)}</p>
				{:else if lastResult?.success}
					<p class="text-2xl font-semibold text-emerald-500">Idle · last run succeeded</p>
					<p class="text-sm text-muted-foreground">Finished {formatTime(lastResult.finishedAt)}</p>
				{:else if lastResult}
					<p class="text-2xl font-semibold text-rose-500">Idle · last run failed</p>
					<p class="text-sm text-muted-foreground">Finished {formatTime(lastResult.finishedAt)}</p>
				{:else}
					<p class="text-2xl font-semibold text-muted-foreground">Idle · no runs yet</p>
				{/if}
			</div>
		</article>

		<article class="space-y-3 rounded-xl border bg-card p-6 shadow-sm">
			<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Repository</p>
			<p class="text-lg font-semibold">{config.repo ?? '—'}</p>
			<p class="text-sm text-muted-foreground">Branch: {config.branch ?? '—'}</p>
		</article>

		<article class="space-y-3 rounded-xl border bg-card p-6 shadow-sm">
			<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">GitHub App webhook</p>
			<p class="text-lg font-semibold">
				{#if config.webhookConfigured}
					Configured
				{:else}
					Not configured
				{/if}
			</p>
			<p class="text-sm text-muted-foreground">
				{#if config.webhookConfigured}
					Listening for pushes to {config.repo ?? 'the configured repo'} on{' '}
					{config.branch ?? 'the configured branch'}.
				{:else}
					Set `GITHUB_APP_WEBHOOK_SECRET` in your environment and mirror it on the GitHub App
					before pushes can trigger updates.
				{/if}
			</p>
		</article>
	</div>

	<section class="grid gap-4 md:grid-cols-2">
		<article class="space-y-3 rounded-xl border bg-card p-6 shadow-sm">
			<header class="flex items-center justify-between">
				<p class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
					Last run
				</p>
			</header>

			{#if lastResult}
				<div class="space-y-1 text-sm">
					<p>Exit code: {lastResult.exitCode ?? '—'}</p>
					<p>Status: {lastResult.success ? 'success' : 'failure'}</p>
					<p>Finished: {formatTime(lastResult.finishedAt)}</p>
					{#if lastResult.error}
						<p class="text-xs text-rose-500">{lastResult.error}</p>
					{/if}
				</div>
			{:else}
				<p class="text-sm text-muted-foreground">No previous updates recorded.</p>
			{/if}
		</article>

		<article class="space-y-3 rounded-xl border bg-card p-6 shadow-sm">
			<header class="flex items-center justify-between">
				<p class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">Trigger</p>
			</header>

			{#if triggerMeta}
				<div class="space-y-1 text-sm">
					<p>Source: {triggerMeta.source ?? 'unknown'}</p>
					{#if triggerMeta.delivery}
						<p>Delivery ID: {triggerMeta.delivery}</p>
					{/if}
					{#if triggerMeta.repo}
						<p>Repo: {triggerMeta.repo}</p>
					{/if}
					{#if triggerMeta.branch}
						<p>Branch: {triggerMeta.branch}</p>
					{/if}
				</div>
			{:else}
				<p class="text-sm text-muted-foreground">
					Waiting for a GitHub webhook to trigger the updater.
				</p>
			{/if}
		</article>
	</section>
</section>
