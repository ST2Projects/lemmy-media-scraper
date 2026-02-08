<script lang="ts">
	import { X, ExternalLink, ArrowUp, Download, User, Calendar } from 'lucide-svelte';
	import { formatFileSize, formatDate, getComments } from '$lib/api';
	import type { MediaItem, Comment } from '$lib/api';
	import Comments from './Comments.svelte';

	interface Props {
		item: MediaItem | null;
		onClose: () => void;
	}

	let { item, onClose }: Props = $props();

	let comments = $state<Comment[]>([]);
	let loadingComments = $state(false);
	let videoEl = $state<HTMLVideoElement | null>(null);

	$effect(() => {
		if (item) {
			loadComments(item.id);
		} else {
			comments = [];
		}
	});

	// Pause video when modal closes
	$effect(() => {
		return () => {
			if (videoEl) {
				videoEl.pause();
				videoEl = null;
			}
		};
	});

	async function loadComments(mediaId: number) {
		loadingComments = true;
		try {
			const res = await getComments(mediaId);
			comments = res.comments || [];
		} catch {
			comments = [];
		} finally {
			loadingComments = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) onClose();
	}

	function isVideo(item: MediaItem): boolean {
		return item.media_type === 'video';
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if item}
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/80 p-4 backdrop-blur-sm"
		onclick={handleBackdropClick}
	>
		<div class="relative my-8 w-full max-w-5xl rounded-lg bg-[#1a1a1a] shadow-2xl">
			<button
				class="absolute top-4 right-4 z-10 rounded-full bg-black/50 p-2 text-white transition-colors hover:bg-black/70"
				onclick={onClose}
			>
				<X class="h-5 w-5" />
			</button>

			<!-- Media display -->
			<div class="flex items-center justify-center bg-black">
				{#if isVideo(item)}
					<video
						bind:this={videoEl}
						src={item.serve_url}
						controls
						class="max-h-[70vh] w-full"
						preload="metadata"
					>
						<track kind="captions" />
					</video>
				{:else}
					<img
						src={item.serve_url}
						alt={item.post_title}
						class="max-h-[70vh] object-contain"
					/>
				{/if}
			</div>

			<!-- Metadata -->
			<div class="border-t border-[#333] p-6">
				<h2 class="mb-4 text-lg font-semibold text-[#e0e0e0]">{item.post_title}</h2>

				<div class="mb-4 grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
					<div class="flex items-center gap-2 text-[#999]">
						<User class="h-4 w-4" />
						<span>{item.author_name}</span>
					</div>
					<div class="flex items-center gap-2 text-[#999]">
						<span class="rounded bg-[#2a2a2a] px-2 py-0.5 text-xs">{item.community_name}</span>
					</div>
					<div class="flex items-center gap-2 text-[#999]">
						<ArrowUp class="h-4 w-4" />
						<span>{item.post_score} points</span>
					</div>
					<div class="flex items-center gap-2 text-[#999]">
						<Download class="h-4 w-4" />
						<span>{formatFileSize(item.file_size)}</span>
					</div>
					<div class="flex items-center gap-2 text-[#999]">
						<Calendar class="h-4 w-4" />
						<span>Posted {formatDate(item.post_created)}</span>
					</div>
					<div class="flex items-center gap-2 text-[#999]">
						<Calendar class="h-4 w-4" />
						<span>Downloaded {formatDate(item.downloaded_at)}</span>
					</div>
					<div class="col-span-2 flex items-center gap-2">
						<a
							href={item.post_url}
							target="_blank"
							rel="noopener noreferrer"
							class="flex items-center gap-1 text-[#6366f1] transition-colors hover:text-[#818cf8]"
						>
							<ExternalLink class="h-4 w-4" />
							View on Lemmy
						</a>
						<a
							href={item.serve_url}
							download={item.file_name}
							class="flex items-center gap-1 text-[#6366f1] transition-colors hover:text-[#818cf8]"
						>
							<Download class="h-4 w-4" />
							Download
						</a>
					</div>
				</div>

				<!-- Comments -->
				<div class="border-t border-[#333] pt-4">
					<h3 class="mb-3 text-sm font-semibold text-[#e0e0e0]">Comments</h3>
					{#if loadingComments}
						<p class="text-sm text-[#666]">Loading comments...</p>
					{:else if comments.length === 0}
						<p class="text-sm text-[#666]">No comments</p>
					{:else}
						<Comments {comments} />
					{/if}
				</div>
			</div>
		</div>
	</div>
{/if}
