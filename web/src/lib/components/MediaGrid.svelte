<script lang="ts">
	import type { MediaItem } from '$lib/api';
	import MediaCard from './MediaCard.svelte';

	interface Props {
		items: MediaItem[];
		onSelect: (item: MediaItem) => void;
	}

	let { items, onSelect }: Props = $props();
</script>

{#if items.length === 0}
	<div class="py-20 text-center text-[#666]">
		<p class="text-lg">No media found</p>
		<p class="mt-1 text-sm">Try adjusting your filters or search query</p>
	</div>
{:else}
	<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
		{#each items as item (item.id)}
			<MediaCard {item} onclick={() => onSelect(item)} />
		{/each}
	</div>
{/if}
