<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { Images, BarChart3, Settings, LogIn, LogOut } from 'lucide-svelte';
	import { authClient } from '$lib/auth-client';
	import { goto, invalidateAll } from '$app/navigation';
	import ProgressBar from '$lib/components/ProgressBar.svelte';

	let { children, data } = $props();

	const publicNavItems = [
		{ href: '/', label: 'Media', icon: Images },
		{ href: '/stats', label: 'Stats', icon: BarChart3 }
	];

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		return page.url.pathname.startsWith(href);
	}

	async function handleLogout() {
		await authClient.signOut();
		await invalidateAll();
		goto('/');
	}
</script>

<div class="flex min-h-screen flex-col">
	<header class="sticky top-0 z-40 border-b border-[#333] bg-[#0f0f0f]/95 backdrop-blur">
		<div class="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
			<a href="/" class="text-lg font-bold text-[#e0e0e0]">
				Lemmy Media Scraper
			</a>

			<nav class="flex items-center gap-1">
				{#each publicNavItems as item}
					<a
						href={item.href}
						class="flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors {isActive(item.href)
							? 'bg-[#6366f1]/10 text-[#6366f1]'
							: 'text-[#999] hover:bg-[#1a1a1a] hover:text-[#e0e0e0]'}"
					>
						<item.icon class="h-4 w-4" />
						{item.label}
					</a>
				{/each}

				{#if data.session}
					<a
						href="/settings"
						class="flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors {isActive('/settings')
							? 'bg-[#6366f1]/10 text-[#6366f1]'
							: 'text-[#999] hover:bg-[#1a1a1a] hover:text-[#e0e0e0]'}"
					>
						<Settings class="h-4 w-4" />
						Settings
					</a>

					<button
						onclick={handleLogout}
						class="ml-2 flex items-center gap-2 rounded-md px-3 py-2 text-sm text-[#999] transition-colors hover:bg-[#1a1a1a] hover:text-[#e0e0e0]"
					>
						<LogOut class="h-4 w-4" />
						Logout
					</button>
				{:else}
					<a
						href="/login"
						class="ml-2 flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors {isActive('/login')
							? 'bg-[#6366f1]/10 text-[#6366f1]'
							: 'text-[#999] hover:bg-[#1a1a1a] hover:text-[#e0e0e0]'}"
					>
						<LogIn class="h-4 w-4" />
						Login
					</a>
				{/if}
			</nav>
		</div>
	</header>

	<div class="mx-auto w-full max-w-7xl px-4 py-2">
		<ProgressBar />
	</div>

	<main class="mx-auto w-full max-w-7xl flex-1 px-4 py-4">
		{@render children()}
	</main>

	<footer class="border-t border-[#333] py-4 text-center text-xs text-[#666]">
		Lemmy Media Scraper
	</footer>
</div>
