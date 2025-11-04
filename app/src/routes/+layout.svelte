<script>
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Button } from 'bits-ui';
	import {
		Menu,
		LayoutDashboard,
		Box,
		Database,
		Cog,
		SunMedium,
		Moon,
		PackagePlus

	} from '@lucide/svelte';

	let { children } = $props();
	let isSidebarOpen = $state(false);
	let isDark = $state(false);

	const navItems = [
		{ label: 'Dashboard', href: '/', icon: LayoutDashboard },
		{ label: 'Deployments', href: '/deployments', icon: Box },
		{ label: 'Databases', href: '/databases', icon: Database },
		{ label: 'System', href: '/system', icon: Cog }
	];

	function toggleSidebar() {
		isSidebarOpen = !isSidebarOpen;
	}

	function toggleTheme() {
		isDark = !isDark;
		if (typeof document === 'undefined') return;
		document.documentElement.classList.toggle('dark', isDark);
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class={`min-h-screen bg-background text-foreground ${isDark ? 'dark' : ''}`}>
	<div class="flex min-h-screen">
		<button
			class="absolute left-4 top-4 flex h-10 w-10 items-center justify-center rounded-lg border bg-background shadow md:hidden"
			type="button"
			onclick={toggleSidebar}
			aria-label="Toggle navigation"
		>
			<Menu class="h-5 w-5" />
		</button>

		<aside
			class={`fixed inset-y-0 z-40 w-72 transform border-r bg-sidebar px-6 py-8 shadow-lg transition-transform duration-200 md:static md:translate-x-0 ${
				isSidebarOpen ? 'translate-x-0' : '-translate-x-full'
			}`}
		>
			<div class="flex items-center justify-between">
				<div>
					<p class="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
						Bakery
					</p>
					<p class="text-lg font-bold">Control plane</p>
				</div>
				<Button.Root variant="ghost" size="icon" onclick={toggleTheme} aria-label="Toggle theme">
					{#if isDark}
						<SunMedium class="h-4 w-4" />
					{:else}
						<Moon class="h-4 w-4" />
					{/if}
				</Button.Root>
			</div>

			<nav class="mt-10 space-y-2">
				{#each navItems as item (item.href)}
					{@const Icon = item.icon}
					<a
						href={item.href}
						class="flex items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium text-muted-foreground transition hover:bg-secondary hover:text-foreground"
						onclick={() => {
							isSidebarOpen = false;
						}}
					>
						<Icon class="h-4 w-4" />
						<span>{item.label}</span>
					</a>
				{/each}
			</nav>

			<div class="mt-10 rounded-lg border bg-background/70 p-4">
				<p class="text-xs uppercase tracking-wide text-muted-foreground">Getting started</p>
				<p class="mt-2 text-sm text-muted-foreground">
					Link GitHub to unlock repository-driven deployments, configure Nginx, and request SSL
					in a few clicks.
				</p>
				<Button.Root class="mt-4 w-full">New deployment</Button.Root>
			</div>
		</aside>

		<div class="flex flex-1 flex-col">
			<header class="sticky top-0 z-30 flex items-center justify-between border-b bg-background/95 px-6 py-4 backdrop-blur supports-[backdrop-filter]:bg-background/60">
				<div class="hidden md:block">
					<h1 class="text-xl font-semibold">Bakery</h1>
					<p class="text-sm text-muted-foreground">Self-hosted deployments without the CLI.</p>
				</div>
				<div class="flex items-center gap-3">
					<Button.Root variant="outline" class="hidden md:flex">
						View activity
					</Button.Root>
					<Button.Root class="gap-2">
						New deployment
						<PackagePlus class="h-4 w-4" />
					</Button.Root>
				</div>
			</header>

			<main class="flex-1 overflow-y-auto bg-background">
				{@render children()}
			</main>
		</div>
	</div>
</div>
