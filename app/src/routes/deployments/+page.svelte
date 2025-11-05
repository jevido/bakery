<script>
	import { Button } from '$lib/components/ui/button';
	import { BadgeCheck, Clock, Globe, AlertTriangle, PackagePlus } from '@lucide/svelte';

	let { data } = $props();
	let deployments = $derived(data.deployments ?? []);

	function formatDate(value) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString();
	}

	function normalizeDomains(domains) {
		if (!domains) return [];
		if (typeof domains === 'string') {
			try {
				return JSON.parse(domains);
			} catch {
				return [];
			}
		}
		return domains;
	}
</script>

<svelte:head>
	<title>Deployments ~ The Bakery</title>
</svelte:head>

<section class="space-y-6 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">Deployments</h1>
			<p class="text-sm text-muted-foreground">
				View deployment status, manage blue-green slots, and trigger rebuilds.
			</p>
		</div>
		<Button class="gap-2" href="/deployments/new">
			<PackagePlus class="h-4 w-4" />
			New deployment
		</Button>
	</header>

	{#if deployments.length === 0}
		<div
			class="flex flex-col items-center justify-center gap-4 rounded-2xl border border-dashed p-12 text-center"
		>
			<div class="flex h-12 w-12 items-center justify-center rounded-full bg-secondary">
				<PackagePlus class="h-6 w-6 text-secondary-foreground" />
			</div>
			<div class="space-y-1">
				<h2 class="text-lg font-semibold">No deployments yet</h2>
				<p class="text-sm text-muted-foreground">
					Run through the deployment wizard to connect your repository, configure env vars, and
					launch your first app.
				</p>
			</div>
			<Button href="/deployments/new">Start deployment wizard</Button>
		</div>
	{:else}
		<div class="overflow-hidden rounded-2xl border">
			<table class="min-w-full divide-y divide-border bg-card">
				<thead class="bg-muted/50">
					<tr class="text-left text-xs font-medium tracking-wide text-muted-foreground uppercase">
						<th class="px-6 py-3">Deployment</th>
						<th class="px-6 py-3">Repository</th>
						<th class="px-6 py-3">Status</th>
						<th class="px-6 py-3">Domains</th>
						<th class="px-6 py-3">Updated</th>
						<th class="px-6 py-3"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-border text-sm">
					{#each deployments as deployment (deployment.id)}
						{@const domains = normalizeDomains(deployment.domains)}
						<tr class="hover:bg-muted/30">
							<td class="px-6 py-4">
								<div class="font-medium text-foreground">{deployment.name}</div>
								<p class="text-xs text-muted-foreground">{deployment.id}</p>
							</td>
							<td class="px-6 py-4">
								<p class="font-medium">{deployment.repository}</p>
								<p class="text-xs text-muted-foreground">Branch {deployment.branch}</p>
							</td>
							<td class="px-6 py-4">
								<div class="flex items-center gap-2">
									{#if deployment.status === 'active'}
										<BadgeCheck class="h-4 w-4 text-emerald-500" />
									{:else if deployment.status === 'failed'}
										<AlertTriangle class="h-4 w-4 text-rose-500" />
									{:else}
										<Clock class="h-4 w-4 text-amber-500" />
									{/if}
									<span class="capitalize">{deployment.status}</span>
								</div>
								{#if deployment.active_slot}
									<p class="text-xs text-muted-foreground">Slot {deployment.active_slot}</p>
								{/if}
							</td>
							<td class="px-6 py-4">
								{#if domains.length === 0}
									<p class="text-xs text-muted-foreground">No domains</p>
								{:else}
									<ul class="space-y-1 text-xs text-muted-foreground">
										{#each domains.slice(0, 3) as domain (domain.id)}
											<li class="flex items-center gap-2">
												<Globe class="h-3.5 w-3.5 text-muted-foreground" />
												<span>{domain.hostname}</span>
											</li>
										{/each}
										{#if domains.length > 3}
											<li>+{domains.length - 3} more</li>
										{/if}
									</ul>
								{/if}
							</td>
							<td class="px-6 py-4 text-xs text-muted-foreground">
								{formatDate(deployment.updated_at)}
							</td>
							<td class="px-6 py-4 text-right">
								<Button
									variant="link"
									class="h-auto p-0 text-sm"
									href={`/deployments/${deployment.id}`}
								>
									View details
								</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
