<script>
	import { Button } from "$lib/components/ui/button/index.js"
	import { Separator } from "$lib/components/ui/separator/index.js"
	
	import { ArrowRight, GitBranch, Database, Server, Link as LinkIcon, PackagePlus } from '@lucide/svelte';

	let stats = $state([
		{
			icon: Server,
			label: 'Active deployments',
			value: '0',
			caption: 'Nothing deployed yet — create your first app.'
		},
		{
			icon: GitBranch,
			label: 'Pending builds',
			value: '0',
			caption: 'New builds appear here while Bakery works.'
		},
		{
			icon: Database,
			label: 'Managed databases',
			value: '0',
			caption: 'Provision a Postgres database directly from a deployment.'
		},
		{
			icon: ArrowRight,
			label: 'Domains secured',
			value: '0',
			caption: 'Attach domains to trigger automatic Nginx + Certbot wiring.'
		}
	]);

	let activity = $state([
		{
			title: 'Link your GitHub account',
			description: 'Connect Bakery to GitHub to unlock repository-driven deployments.',
			action: 'Open GitHub settings'
		},
		{
			title: 'Seed your first deployment',
			description: 'Use the “New deployment” button to walk through the build wizard.',
			action: 'Start deployment wizard'
		},
		{
			title: 'Add a custom domain',
			description:
				'Attach a domain to view DNS guidance, SSL state, and blue-green slot mapping.',
			action: 'Go to domain manager'
		}
	]);
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
			<Button.Root variant="outline" size="icon">
				<LinkIcon class="h-4 w-4" />
				Link GitHub
			</Button.Root>
			<Button.Root size="icon">
				New deployment
				<PackagePlus class="h-4 w-4" />
			</Button.Root>
		</div>
	</div>

	<Separator.Root />

	<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
		{#each stats as stat (stat.label)}
			{@const Icon = stat.icon}
			<div class="rounded-xl border bg-card p-4 shadow-sm">
				<div class="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
					<Icon class="h-5 w-5 text-secondary-foreground" />
				</div>
				<p class="text-sm text-muted-foreground">{stat.label}</p>
				<p class="mt-2 text-2xl font-semibold">{stat.value}</p>
				<p class="mt-1 text-xs text-muted-foreground">{stat.caption}</p>
			</div>
		{/each}
	</div>

	<div class="grid gap-4 lg:grid-cols-[2fr,1fr]">
		<section class="rounded-xl border bg-card p-6 shadow-sm">
			<header class="flex items-center justify-between">
				<div>
					<h2 class="text-lg font-semibold">Deployment readiness</h2>
					<p class="text-sm text-muted-foreground">
						Get started by working through the onboarding checklist below.
					</p>
				</div>
				<Button.Root variant="ghost" class="text-primary hover:text-primary">
					View tasks
				</Button.Root>
			</header>
			<ul class="mt-6 space-y-4">
				{#each activity as item (item.title)}
					<li class="rounded-lg border bg-background/40 p-4">
						<p class="text-sm font-medium">{item.title}</p>
						<p class="mt-1 text-sm text-muted-foreground">{item.description}</p>
						<Button.Root variant="link" class="mt-3 h-auto p-0 text-sm">
							{item.action}
						</Button.Root>
					</li>
				{/each}
			</ul>
		</section>

		<aside class="rounded-xl border bg-card p-6 shadow-sm">
			<h2 class="text-lg font-semibold">Live status</h2>
			<p class="text-sm text-muted-foreground">
				Status lights turn green as Bakery connects to GitHub, Postgres, and Nginx.
			</p>
			<ul class="mt-6 space-y-3">
				<li class="flex items-center justify-between rounded-lg border bg-background/40 p-3">
					<span class="text-sm font-medium">GitHub</span>
					<span class="rounded-full border border-primary/40 bg-primary/10 px-2 py-0.5 text-xs text-primary">
						Pending setup
					</span>
				</li>
				<li class="flex items-center justify-between rounded-lg border bg-background/40 p-3">
					<span class="text-sm font-medium">PostgreSQL</span>
					<span class="rounded-full border border-primary/40 bg-primary/10 px-2 py-0.5 text-xs text-primary">
						Pending setup
					</span>
				</li>
				<li class="flex items-center justify-between rounded-lg border bg-background/40 p-3">
					<span class="text-sm font-medium">Nginx &amp; Certbot</span>
					<span class="rounded-full border border-primary/40 bg-primary/10 px-2 py-0.5 text-xs text-primary">
						Pending setup
					</span>
				</li>
			</ul>
		</aside>
	</div>
</section>
