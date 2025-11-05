<script>
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Card from '$lib/components/ui/card';

	import { goto } from '$app/navigation';
	import {
		ArrowRight,
		Server,
		GitBranch,
		Database,
		Globe,
		CheckCircle,
		Circle,
		Link as LinkIcon,
		HardDrive,
		Activity
	} from '@lucide/svelte';

	let { data } = $props();

	let analytics = $derived(data.analytics ?? {});
	let deployments = $derived(data.deployments ?? []);
	let deploymentStats = $derived(analytics.deploymentStats ?? { total: 0, active: 0, pending: 0 });
	let databaseStats = $derived(analytics.databaseStats ?? { total: 0 });
	let domainStats = $derived(analytics.domainStats ?? { total: 0, verified: 0 });
	let tasks = $derived(analytics.tasks ?? []);
	let disk = $derived(analytics.disk);
	let systemDisk = $derived(analytics.systemDisk);

	const stats = $derived([
		{
			icon: Server,
			label: 'Active deployments',
			value: String(deploymentStats.active ?? 0),
			caption: `${deploymentStats.total ?? 0} total`
		},
		{
			icon: GitBranch,
			label: 'Pending builds',
			value: String(deploymentStats.pending ?? 0),
			caption: (deploymentStats.pending ?? 0) > 0 ? 'Builds queued or running' : 'All caught up'
		},
		{
			icon: Database,
			label: 'Managed databases',
			value: String(databaseStats.total ?? 0),
			caption:
				(databaseStats.total ?? 0) > 0 ? 'Provisioned via Bakery' : 'Create from a deployment'
		},
		{
			icon: Globe,
			label: 'Domains secured',
			value: String(domainStats.verified ?? 0),
			caption:
				(domainStats.total ?? 0) > 0
					? `${domainStats.total} total domains`
					: 'Add a domain to issue SSL'
		}
	]);

	const checklist = $derived([
		{
			title: 'Link your GitHub account',
			description: 'Connect Bakery to GitHub to unlock repository-driven deployments.',
			done: Boolean(analytics.githubLinked),
			actionLabel: 'Link GitHub',
			action: startGithubLink
		},
		{
			title: 'Create your first deployment',
			description: 'Use the deployment wizard to configure repository, env vars, and domains.',
			done: deployments.length > 0,
			actionLabel: 'New deployment',
			action: () => goto('/deployments/new')
		},
		{
			title: 'Attach a custom domain',
			description: 'Point DNS to your server and let Bakery provision SSL automatically.',
			done: (domainStats.total ?? 0) > 0,
			actionLabel: 'Manage domains',
			action: () => goto('/deployments')
		}
	]);

	const statusCards = $derived([
		{
			label: 'GitHub',
			ok: Boolean(analytics.githubLinked),
			message: analytics.githubLinked
				? 'GitHub account linked'
				: 'Link GitHub to deploy from repositories',
			action: analytics.githubLinked ? null : startGithubLink
		},
		{
			label: 'PostgreSQL',
			ok: true,
			message:
				(databaseStats.total ?? 0) > 0
					? `${databaseStats.total} managed database${databaseStats.total === 1 ? '' : 's'}`
					: 'Ready to provision databases on demand',
			action: () => goto('/deployments')
		},
		{
			label: 'Nginx & Certbot',
			ok: (domainStats.verified ?? 0) > 0,
			message:
				(domainStats.verified ?? 0) > 0
					? `${domainStats.verified} certificate${domainStats.verified === 1 ? '' : 's'} active`
					: 'Add a domain to issue certificates automatically',
			action: () => goto('/deployments')
		}
	]);

	const recentDeployments = $derived(deployments.slice(0, 5));

	function formatBytes(bytes) {
		if (bytes == null) return '—';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let value = Number(bytes);
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		return `${value.toFixed(1)} ${units[unitIndex]}`;
	}

	function formatDate(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString();
	}

	async function startGithubLink() {
		try {
			const response = await fetch('/api/auth/github/url', { credentials: 'include' });
			if (response.ok) {
				const payload = await response.json();
				if (payload.url) {
					window.location.href = payload.url;
				}
			}
		} catch {
			// ignore
		}
	}
</script>

<section class="space-y-8 p-6 md:p-10">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Bakery Control Center</h1>
			<p class="text-muted-foreground">
				Orchestrate deployments, databases, domains, and SSL certificates without leaving the
				browser.
			</p>
		</div>
		<div class="flex gap-3">
			<Button variant="outline" class="gap-2 px-4" onclick={startGithubLink}>
				<LinkIcon class="h-4 w-4" />
				Link GitHub
			</Button>
			<Button class="gap-2 px-4" onclick={() => goto('/deployments/new')}>
				New deployment
				<ArrowRight class="h-4 w-4" />
			</Button>
		</div>
	</div>

	<Separator />

	<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
		{#each stats as stat (stat.label)}
			{@const Icon = stat.icon}
			<Card.Root>
				<Card.Header class="pb-0">
					<header class="flex items-center gap-3">
						<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
							<Icon class="h-5 w-5 text-secondary-foreground" />
						</div>
						<div>
							<p class="text-sm font-semibold">{stat.label}</p>
							<p class="text-xs text-muted-foreground">on this server</p>
						</div>
					</header>
				</Card.Header>
				<Card.Content class="flex mt-0 right-0 items-center justify-between">
					<p class="mt-2 text-2xl font-semibold">{stat.value}</p>
					<p class="mt-1 text-xs text-muted-foreground">{stat.caption}</p>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>

	<div class="grid gap-4 lg:grid-cols-[2fr,1fr]">
		<section class="rounded-xl border bg-card p-6 shadow-sm">
			<header class="flex items-center justify-between">
				<div>
					<h2 class="text-lg font-semibold">Deployment readiness</h2>
					<p class="text-sm text-muted-foreground">
						Work through the onboarding checklist to unlock a zero-touch deployment workflow.
					</p>
				</div>
			</header>
			<ul class="mt-6 space-y-4">
				{#each checklist as item (item.title)}
					<li class="rounded-lg border bg-background/40 p-4">
						<div class="flex items-start justify-between gap-3">
							<div class="flex items-start gap-3">
								{#if item.done}
									<CheckCircle class="mt-0.5 h-4 w-4 text-emerald-500" />
								{:else}
									<Circle class="mt-0.5 h-4 w-4 text-muted-foreground" />
								{/if}
								<div>
									<p class="text-sm font-medium">{item.title}</p>
									<p class="mt-1 text-sm text-muted-foreground">{item.description}</p>
								</div>
							</div>
							{#if !item.done}
								<Button variant="link" class="h-auto p-0 text-sm" onclick={item.action}>
									{item.actionLabel}
								</Button>
							{/if}
						</div>
					</li>
				{/each}
			</ul>

			<div class="mt-8 grid gap-4 lg:grid-cols-2">
				<div class="rounded-lg border bg-background/50 p-4">
					<div class="flex items-center gap-2 text-sm font-medium">
						<HardDrive class="h-4 w-4 text-muted-foreground" />
						Build storage
					</div>
					<p class="mt-2 text-2xl font-semibold">
						{disk ? formatBytes(disk.used) : '—'}
						<span class="text-sm text-muted-foreground">used</span>
					</p>
					<p class="text-xs text-muted-foreground">
						Capacity {disk ? formatBytes(disk.total) : '—'} · Free {disk
							? formatBytes(disk.free)
							: '—'}
					</p>
				</div>
				<div class="rounded-lg border bg-background/50 p-4">
					<div class="flex items-center gap-2 text-sm font-medium">
						<Activity class="h-4 w-4 text-muted-foreground" />
						System disk
					</div>
					<p class="mt-2 text-2xl font-semibold">
						{systemDisk ? formatBytes(systemDisk.used) : '—'}
						<span class="text-sm text-muted-foreground">used</span>
					</p>
					<p class="text-xs text-muted-foreground">
						Capacity {systemDisk ? formatBytes(systemDisk.total) : '—'} · Free {systemDisk
							? formatBytes(systemDisk.free)
							: '—'}
					</p>
				</div>
			</div>
		</section>

		<aside class="space-y-4">
			<div class="rounded-xl border bg-card p-6 shadow-sm">
				<h2 class="text-lg font-semibold">Platform status</h2>
				<p class="text-sm text-muted-foreground">
					Status lights turn green as integrations and services come online.
				</p>
				<ul class="mt-6 space-y-3">
					{#each statusCards as card (card.label)}
						<li class="rounded-lg border bg-background/40 p-3">
							<div class="flex items-center justify-between gap-3">
								<div>
									<p class="text-sm font-medium">{card.label}</p>
									<p class="text-xs text-muted-foreground">{card.message}</p>
								</div>
								<span
									class={`rounded-full px-2 py-0.5 text-xs ${
										card.ok
											? 'border border-emerald-400/40 bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200'
											: 'border border-amber-400/40 bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-200'
									}`}
								>
									{card.ok ? 'Ready' : 'Action needed'}
								</span>
							</div>
							{#if card.action && !card.ok}
								<Button variant="link" class="mt-2 h-auto p-0 text-sm" onclick={card.action}>
									Resolve
								</Button>
							{/if}
						</li>
					{/each}
				</ul>
			</div>

			<div class="rounded-xl border bg-card p-6 shadow-sm">
				<h2 class="text-lg font-semibold">Recent tasks</h2>
				<p class="text-sm text-muted-foreground">
					Builds, restarts, and analytics runs executed by the task worker.
				</p>
				{#if tasks.length === 0}
					<p class="mt-4 text-sm text-muted-foreground">
						No tasks yet. Kick off a deployment to get started.
					</p>
				{:else}
					<ul class="mt-4 space-y-3">
						{#each tasks.slice(0, 5) as task (task.id)}
							<li class="rounded-lg border bg-background/50 p-3">
								<div class="flex items-center justify-between">
									<p class="text-sm font-medium capitalize">{task.type}</p>
									<span
										class={`text-xs font-medium ${
											task.status === 'completed'
												? 'text-emerald-500'
												: task.status === 'failed'
													? 'text-rose-500'
													: 'text-amber-500'
										}`}
									>
										{task.status}
									</span>
								</div>
								<p class="mt-1 text-xs text-muted-foreground">{formatDate(task.updated_at)}</p>
							</li>
						{/each}
					</ul>
				{/if}
			</div>

			{#if recentDeployments.length}
				<div class="rounded-xl border bg-card p-6 shadow-sm">
					<h2 class="text-lg font-semibold">Recent deployments</h2>
					<ul class="mt-4 space-y-3">
						{#each recentDeployments as deployment (deployment.id)}
							<li class="rounded-lg border bg-background/50 p-3">
								<div class="flex items-center justify-between gap-3">
									<div>
										<p class="text-sm font-medium">{deployment.name}</p>
										<p class="text-xs text-muted-foreground">
											{deployment.repository} · {deployment.branch}
										</p>
									</div>
									<span
										class={`text-xs font-medium ${
											deployment.status === 'active'
												? 'text-emerald-500'
												: deployment.status === 'failed'
													? 'text-rose-500'
													: 'text-amber-500'
										}`}
									>
										{deployment.status}
									</span>
								</div>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</aside>
	</div>
</section>
