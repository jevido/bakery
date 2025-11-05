<script>
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { createUserAccount, updateUserAccount, deleteUserAccount } from '$lib/api.js';
	import { UserPlus, ShieldCheck, ShieldOff, Lock, Trash2 } from '@lucide/svelte';

	let { data } = $props();
	let users = $state(data.users ?? []);
	let form = $state({ email: '', password: '', confirm: '', isAdmin: false });
	let formError = $state('');
	let submitting = $state(false);
	let userMessage = $state('');
	let busyUserId = $state(null);

	function resetForm() {
		form = { email: '', password: '', confirm: '', isAdmin: false };
		formError = '';
	}

	async function handleCreate(event) {
		event.preventDefault();
		formError = '';
		userMessage = '';
		const email = form.email.trim();
		if (!email) {
			formError = 'Email is required.';
			return;
		}
		if (form.password.length < 8) {
			formError = 'Password must be at least 8 characters.';
			return;
		}
		if (form.password !== form.confirm) {
			formError = 'Passwords do not match.';
			return;
		}
		submitting = true;
		try {
			const payload = await createUserAccount({
				email,
				password: form.password,
				isAdmin: form.isAdmin
			});
			if (payload?.user) {
				users = [payload.user, ...users.filter((user) => user.id !== payload.user.id)];
				userMessage = `User ${payload.user.email} created.`;
				resetForm();
			}
		} catch (err) {
			formError = err?.message ?? 'Failed to create user.';
		} finally {
			submitting = false;
		}
	}

	async function toggleAdmin(user) {
		busyUserId = user.id;
		userMessage = '';
		try {
			const payload = await updateUserAccount(user.id, { isAdmin: !user.is_admin });
			if (payload?.user) {
				users = users.map((row) => (row.id === user.id ? payload.user : row));
			}
		} catch (err) {
			userMessage = err?.message ?? 'Failed to update user.';
		} finally {
			busyUserId = null;
		}
	}

	async function resetPassword(user) {
		const password = window.prompt(`Enter a new password for ${user.email} (min 8 chars)`);
		if (!password) return;
		if (password.length < 8) {
			window.alert('Password must be at least 8 characters.');
			return;
		}
		busyUserId = user.id;
		userMessage = '';
		try {
			await updateUserAccount(user.id, { password });
			userMessage = 'Password updated.';
		} catch (err) {
			userMessage = err?.message ?? 'Failed to reset password.';
		} finally {
			busyUserId = null;
		}
	}

	async function removeUser(user) {
		if (!window.confirm(`Delete ${user.email}? This cannot be undone.`)) {
			return;
		}
		busyUserId = user.id;
		userMessage = '';
		try {
			await deleteUserAccount(user.id);
			users = users.filter((row) => row.id !== user.id);
			userMessage = `User ${user.email} removed.`;
		} catch (err) {
			userMessage = err?.message ?? 'Failed to delete user.';
		} finally {
			busyUserId = null;
		}
	}
</script>

<section class="space-y-8 p-6 md:p-10">
	<header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-semibold tracking-tight">User management</h1>
			<p class="text-sm text-muted-foreground">
				Invite teammates, promote administrators, and manage credentials.
			</p>
		</div>
	</header>

	<form class="rounded-xl border bg-card p-6 shadow-sm" onsubmit={handleCreate}>
		<header class="mb-6 flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
				<UserPlus class="h-5 w-5 text-secondary-foreground" />
			</div>
			<div>
				<h2 class="text-lg font-semibold">Create a new user</h2>
				<p class="text-sm text-muted-foreground">Set an email, password, and optional admin role.</p>
			</div>
		</header>
		<div class="grid gap-4 md:grid-cols-2">
			<div class="space-y-2">
				<Label for="new-email">Email</Label>
				<Input
					id="new-email"
					type="email"
					required
					autocomplete="off"
					value={form.email}
					on:input={(event) => {
						form = { ...form, email: event.currentTarget.value };
					}}
				/>
			</div>
			<div class="space-y-2">
				<Label for="new-password">Password</Label>
				<Input
					id="new-password"
					type="password"
					required
					autocomplete="new-password"
					minlength={8}
					value={form.password}
					on:input={(event) => {
						form = { ...form, password: event.currentTarget.value };
					}}
				/>
			</div>
			<div class="space-y-2">
				<Label for="confirm-password">Confirm password</Label>
				<Input
					id="confirm-password"
					type="password"
					required
					autocomplete="new-password"
					minlength={8}
					value={form.confirm}
					on:input={(event) => {
						form = { ...form, confirm: event.currentTarget.value };
					}}
				/>
			</div>
			<div class="flex items-center justify-between rounded-lg border bg-background/70 px-4 py-3">
				<div>
					<p class="text-sm font-medium">Administrator</p>
					<p class="text-xs text-muted-foreground">Grant full access to user management.</p>
				</div>
				<Switch
					checked={form.isAdmin}
					on:change={(event) => {
						form = { ...form, isAdmin: event.currentTarget.checked };
					}}
				/>
			</div>
		</div>
		{#if formError}
			<p class="mt-4 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/40 dark:bg-rose-900/30 dark:text-rose-200">
				{formError}
			</p>
		{/if}
		<div class="mt-6 flex justify-end">
			<Button type="submit" disabled={submitting}>
				{#if submitting}
					Creating...
				{:else}
					Create user
				{/if}
			</Button>
		</div>
	</form>

	{#if userMessage}
		<p class="rounded border border-emerald-200 bg-emerald-50 px-4 py-2 text-sm text-emerald-700 dark:border-emerald-900/40 dark:bg-emerald-900/20 dark:text-emerald-200">
			{userMessage}
		</p>
	{/if}

	<section class="overflow-hidden rounded-xl border bg-card shadow-sm">
		<table class="min-w-full divide-y divide-border">
			<thead class="bg-muted/50">
				<tr class="text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
					<th class="px-6 py-3">User</th>
					<th class="px-6 py-3">Role</th>
					<th class="px-6 py-3">Deployments</th>
					<th class="px-6 py-3">Created</th>
					<th class="px-6 py-3 text-right">Actions</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-border text-sm">
				{#each users as user (user.id)}
					<tr class="hover:bg-muted/30">
						<td class="px-6 py-4">
							<div class="font-medium text-foreground">{user.email}</div>
							{#if user.is_initial}
								<p class="text-xs text-muted-foreground">Initial administrator</p>
							{/if}
						</td>
						<td class="px-6 py-4">
							{#if user.is_admin}
								<span class="inline-flex items-center gap-2 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-800 dark:bg-emerald-500/15 dark:text-emerald-200">
									<ShieldCheck class="h-3.5 w-3.5" /> Admin
								</span>
							{:else}
								<span class="inline-flex items-center gap-2 rounded-full bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground">
									<ShieldOff class="h-3.5 w-3.5" /> User
								</span>
							{/if}
						</td>
						<td class="px-6 py-4 text-muted-foreground">{user.deployments}</td>
						<td class="px-6 py-4 text-xs text-muted-foreground">{new Date(user.created_at).toLocaleString()}</td>
						<td class="px-6 py-4">
							<div class="flex flex-wrap justify-end gap-2">
								<Button
									variant="outline"
									size="sm"
									disabled={busyUserId === user.id}
									onclick={() => toggleAdmin(user)}
								>
									{user.is_admin ? 'Remove admin' : 'Promote to admin'}
								</Button>
								<Button
									variant="outline"
									size="sm"
									disabled={busyUserId === user.id}
									onclick={() => resetPassword(user)}
								>
									<Lock class="mr-2 h-4 w-4" /> Reset password
								</Button>
								<Button
									variant="destructive"
									size="sm"
									disabled={user.is_initial || busyUserId === user.id}
									onclick={() => removeUser(user)}
								>
									<Trash2 class="mr-2 h-4 w-4" /> Delete
								</Button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</section>
</section>
