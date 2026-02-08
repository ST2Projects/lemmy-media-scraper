<script lang="ts">
	import { Activity, Loader } from 'lucide-svelte';
	import { browser } from '$app/environment';
	import { env } from '$env/dynamic/public';
	import type { ProgressUpdate } from '$lib/api';

	let progress = $state<ProgressUpdate | null>(null);
	let connected = $state(false);

	$effect(() => {
		if (!browser) return;

		// PUBLIC_WS_URL can be set to point directly to the Go backend WebSocket
		// (e.g., ws://localhost:8081) when running as separate containers.
		// When unset, defaults to the current page host (same-origin).
		let wsUrl: string;
		if (env.PUBLIC_WS_URL) {
			wsUrl = `${env.PUBLIC_WS_URL}/ws/progress`;
		} else {
			const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
			wsUrl = `${protocol}//${window.location.host}/ws/progress`;
		}
		const ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			connected = true;
		};

		ws.onmessage = (event) => {
			try {
				progress = JSON.parse(event.data);
			} catch {
				// ignore parse errors
			}
		};

		ws.onclose = () => {
			connected = false;
			progress = null;
		};

		ws.onerror = () => {
			connected = false;
		};

		return () => {
			ws.close();
		};
	});

	function formatEta(seconds: number): string {
		if (seconds <= 0) return '';
		if (seconds < 60) return `${Math.round(seconds)}s`;
		const minutes = Math.floor(seconds / 60);
		const secs = Math.round(seconds % 60);
		return `${minutes}m ${secs}s`;
	}
</script>

{#if progress?.is_running}
	<div class="flex items-center gap-3 rounded-lg bg-[#1a1a1a] border border-[#333] px-4 py-3">
		<Loader class="h-4 w-4 animate-spin text-[#6366f1]" />
		<div class="flex flex-1 items-center gap-4 text-sm">
			<span class="font-medium text-[#e0e0e0]">
				Scraping{progress.community ? ` ${progress.community}` : ''}...
			</span>
			<span class="text-[#999]">
				{progress.posts_processed} posts
			</span>
			<span class="text-green-400">
				{progress.media_downloaded} downloaded
			</span>
			{#if progress.errors > 0}
				<span class="text-red-400">
					{progress.errors} errors
				</span>
			{/if}
			{#if progress.eta_seconds > 0}
				<span class="text-[#666]">
					ETA: {formatEta(progress.eta_seconds)}
				</span>
			{/if}
		</div>
	</div>
{:else if connected}
	<div class="flex items-center gap-2 text-xs text-[#666]">
		<Activity class="h-3 w-3" />
		<span>Connected - waiting for scrape</span>
	</div>
{/if}
