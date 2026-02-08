<script lang="ts">
	import { Image, Video, File, ArrowUp } from 'lucide-svelte';
	import { formatFileSize } from '$lib/api';
	import type { MediaItem } from '$lib/api';

	interface Props {
		item: MediaItem;
		onclick: () => void;
	}

	let { item, onclick }: Props = $props();

	let imgError = $state(false);

	function getTypeColor(type: string): string {
		if (type === 'image') return 'bg-blue-500/20 text-blue-400';
		if (type === 'video') return 'bg-purple-500/20 text-purple-400';
		return 'bg-gray-500/20 text-gray-400';
	}
</script>

<button
	class="group w-full cursor-pointer overflow-hidden rounded-lg border border-[#333] bg-[#1a1a1a] text-left transition-all hover:border-[#6366f1]/50 hover:shadow-lg hover:shadow-[#6366f1]/5"
	{onclick}
>
	<div class="relative aspect-square overflow-hidden bg-[#111]">
		{#if !imgError}
			<img
				src="/thumbnails/{item.id}"
				alt={item.post_title}
				class="h-full w-full object-cover transition-transform group-hover:scale-105"
				loading="lazy"
				onerror={() => (imgError = true)}
			/>
		{:else}
			<div class="flex h-full w-full items-center justify-center">
				{#if item.media_type === 'video'}
					<Video class="h-12 w-12 text-[#666]" />
				{:else if item.media_type === 'image'}
					<Image class="h-12 w-12 text-[#666]" />
				{:else}
					<File class="h-12 w-12 text-[#666]" />
				{/if}
			</div>
		{/if}

		<span
			class="absolute top-2 right-2 rounded px-1.5 py-0.5 text-xs font-medium {getTypeColor(item.media_type)}"
		>
			{item.media_type}
		</span>
	</div>

	<div class="p-3">
		<h3 class="mb-1 truncate text-sm font-medium text-[#e0e0e0]" title={item.post_title}>
			{item.post_title}
		</h3>
		<div class="flex items-center justify-between text-xs text-[#999]">
			<span class="truncate">{item.community_name}</span>
			<div class="flex items-center gap-2">
				<span class="flex items-center gap-0.5">
					<ArrowUp class="h-3 w-3" />
					{item.post_score}
				</span>
				<span>{formatFileSize(item.file_size)}</span>
			</div>
		</div>
	</div>
</button>
