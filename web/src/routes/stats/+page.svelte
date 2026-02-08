<script lang="ts">
	import { getStats, getTimeline, getTopCreators, getStorageBreakdown, formatFileSize } from '$lib/api';
	import type { Stats, TimelineEntry, TopCreator, StorageBreakdown } from '$lib/api';
	import { BarChart3, HardDrive, Users, TrendingUp, Image, Video, File } from 'lucide-svelte';

	let stats = $state<Stats | null>(null);
	let timeline = $state<TimelineEntry[]>([]);
	let topCreators = $state<TopCreator[]>([]);
	let storage = $state<StorageBreakdown | null>(null);
	let timelinePeriod = $state<'day' | 'week' | 'month'>('day');
	let loading = $state(true);

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		loading = true;
		try {
			const [s, tl, tc, st] = await Promise.all([
				getStats(),
				getTimeline(timelinePeriod),
				getTopCreators(10),
				getStorageBreakdown()
			]);
			stats = s;
			timeline = tl || [];
			topCreators = tc || [];
			storage = st;
		} catch {
			// keep defaults
		} finally {
			loading = false;
		}
	}

	async function changeTimelinePeriod(period: 'day' | 'week' | 'month') {
		timelinePeriod = period;
		try {
			timeline = (await getTimeline(period)) || [];
		} catch {
			timeline = [];
		}
	}

	function getMaxTimelineCount(): number {
		if (timeline.length === 0) return 1;
		return Math.max(...timeline.map((t) => t.count), 1);
	}
</script>

<svelte:head>
	<title>Statistics - Lemmy Media Scraper</title>
</svelte:head>

{#if loading}
	<div class="py-20 text-center">
		<div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-[#333] border-t-[#6366f1]"></div>
		<p class="mt-4 text-sm text-[#666]">Loading statistics...</p>
	</div>
{:else}
	<div class="space-y-6">
		<!-- Overview cards -->
		{#if stats}
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
				<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-4">
					<div class="flex items-center gap-2 text-[#999]">
						<BarChart3 class="h-4 w-4" />
						<span class="text-xs uppercase">Total Media</span>
					</div>
					<p class="mt-2 text-2xl font-bold text-[#e0e0e0]">{stats.total_media.toLocaleString()}</p>
				</div>
				<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-4">
					<div class="flex items-center gap-2 text-[#999]">
						<Image class="h-4 w-4" />
						<span class="text-xs uppercase">Images</span>
					</div>
					<p class="mt-2 text-2xl font-bold text-blue-400">{(stats.by_type?.image || 0).toLocaleString()}</p>
				</div>
				<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-4">
					<div class="flex items-center gap-2 text-[#999]">
						<Video class="h-4 w-4" />
						<span class="text-xs uppercase">Videos</span>
					</div>
					<p class="mt-2 text-2xl font-bold text-purple-400">{(stats.by_type?.video || 0).toLocaleString()}</p>
				</div>
				<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-4">
					<div class="flex items-center gap-2 text-[#999]">
						<File class="h-4 w-4" />
						<span class="text-xs uppercase">Other</span>
					</div>
					<p class="mt-2 text-2xl font-bold text-gray-400">{(stats.by_type?.other || 0).toLocaleString()}</p>
				</div>
			</div>
		{/if}

		<!-- Timeline -->
		<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
			<div class="mb-4 flex items-center justify-between">
				<div class="flex items-center gap-2">
					<TrendingUp class="h-5 w-5 text-[#999]" />
					<h2 class="text-lg font-semibold text-[#e0e0e0]">Download Timeline</h2>
				</div>
				<div class="flex gap-1">
					{#each ['day', 'week', 'month'] as period}
						<button
							class="rounded px-3 py-1 text-xs transition-colors {timelinePeriod === period
								? 'bg-[#6366f1] text-white'
								: 'text-[#999] hover:bg-[#2a2a2a]'}"
							onclick={() => changeTimelinePeriod(period as 'day' | 'week' | 'month')}
						>
							{period}
						</button>
					{/each}
				</div>
			</div>

			{#if timeline.length === 0}
				<p class="text-center text-sm text-[#666]">No timeline data available</p>
			{:else}
				<div class="space-y-1">
					{#each timeline.slice(-20) as entry}
						<div class="flex items-center gap-3 text-sm">
							<span class="w-24 shrink-0 text-right text-xs text-[#999]">{entry.period}</span>
							<div class="h-5 flex-1 overflow-hidden rounded bg-[#2a2a2a]">
								<div
									class="h-full rounded bg-[#6366f1] transition-all"
									style="width: {(entry.count / getMaxTimelineCount()) * 100}%"
								></div>
							</div>
							<span class="w-12 text-right text-xs text-[#999]">{entry.count}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="grid gap-6 md:grid-cols-2">
			<!-- Top Creators -->
			<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
				<div class="mb-4 flex items-center gap-2">
					<Users class="h-5 w-5 text-[#999]" />
					<h2 class="text-lg font-semibold text-[#e0e0e0]">Top Creators</h2>
				</div>

				{#if topCreators.length === 0}
					<p class="text-sm text-[#666]">No creator data available</p>
				{:else}
					<div class="space-y-2">
						{#each topCreators as creator, i}
							<div class="flex items-center justify-between rounded bg-[#222] px-3 py-2 text-sm">
								<div class="flex items-center gap-2">
									<span class="w-5 text-right text-xs text-[#666]">{i + 1}</span>
									<span class="text-[#e0e0e0]">{creator.name}</span>
								</div>
								<span class="text-[#999]">{creator.count}</span>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Storage Breakdown -->
			<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
				<div class="mb-4 flex items-center gap-2">
					<HardDrive class="h-5 w-5 text-[#999]" />
					<h2 class="text-lg font-semibold text-[#e0e0e0]">Storage</h2>
				</div>

				{#if storage}
					{#if storage.by_type && storage.by_type.length > 0}
						<h3 class="mb-2 text-xs font-semibold uppercase text-[#999]">By Type</h3>
						<div class="mb-4 space-y-1">
							{#each storage.by_type as entry}
								<div class="flex items-center justify-between rounded bg-[#222] px-3 py-2 text-sm">
									<span class="text-[#e0e0e0]">{entry.type}</span>
									<span class="text-[#999]">{formatFileSize(entry.size)} ({entry.count})</span>
								</div>
							{/each}
						</div>
					{/if}

					{#if storage.by_community && storage.by_community.length > 0}
						<h3 class="mb-2 text-xs font-semibold uppercase text-[#999]">Top Communities</h3>
						<div class="space-y-1">
							{#each storage.by_community.slice(0, 10) as entry}
								<div class="flex items-center justify-between rounded bg-[#222] px-3 py-2 text-sm">
									<span class="truncate text-[#e0e0e0]">{entry.name}</span>
									<span class="shrink-0 text-[#999]">{formatFileSize(entry.size)} ({entry.count})</span>
								</div>
							{/each}
						</div>
					{/if}
				{:else}
					<p class="text-sm text-[#666]">No storage data available</p>
				{/if}
			</div>
		</div>

		<!-- Top Communities by count -->
		{#if stats?.top_communities}
			<div class="rounded-lg border border-[#333] bg-[#1a1a1a] p-6">
				<h2 class="mb-4 text-lg font-semibold text-[#e0e0e0]">Communities by Media Count</h2>
				<div class="grid gap-2 sm:grid-cols-2 md:grid-cols-3">
					{#each Object.entries(stats.top_communities).sort((a, b) => b[1] - a[1]) as [name, count]}
						<div class="flex items-center justify-between rounded bg-[#222] px-3 py-2 text-sm">
							<span class="truncate text-[#e0e0e0]">{name}</span>
							<span class="shrink-0 text-[#999]">{count}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{/if}
