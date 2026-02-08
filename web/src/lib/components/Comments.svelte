<script lang="ts">
	import { ArrowUp, ArrowDown } from 'lucide-svelte';
	import type { Comment } from '$lib/api';

	interface Props {
		comments: Comment[];
	}

	let { comments }: Props = $props();

	interface CommentTree {
		comment: Comment;
		children: CommentTree[];
	}

	function buildTree(comments: Comment[]): CommentTree[] {
		// Path format: "0.parentId.childId.etc"
		// Depth is determined by number of segments in path
		const sorted = [...comments].sort((a, b) => {
			const pathA = a.comment_path || '';
			const pathB = b.comment_path || '';
			return pathA.localeCompare(pathB);
		});

		const roots: CommentTree[] = [];
		const nodeMap = new Map<number, CommentTree>();

		for (const comment of sorted) {
			const node: CommentTree = { comment, children: [] };
			nodeMap.set(comment.comment_id, node);

			const pathParts = (comment.comment_path || '').split('.').filter(Boolean);

			if (pathParts.length <= 2) {
				// Root-level comment (path is "0.commentId")
				roots.push(node);
			} else {
				// Find parent from path - parent ID is second-to-last segment
				const parentId = parseInt(pathParts[pathParts.length - 2]);
				const parent = nodeMap.get(parentId);
				if (parent) {
					parent.children.push(node);
				} else {
					roots.push(node);
				}
			}
		}

		return roots;
	}

	let tree = $derived(buildTree(comments));

	function getDepthClass(depth: number): string {
		if (depth === 0) return '';
		return `ml-4 border-l border-[#333] pl-4`;
	}

	function formatScore(score: number): string {
		if (score >= 1000) return `${(score / 1000).toFixed(1)}k`;
		return String(score);
	}
</script>

{#snippet commentNode(nodes: CommentTree[], depth: number)}
	{#each nodes as node}
		<div class={getDepthClass(depth)}>
			<div class="mb-3 rounded bg-[#222] p-3">
				<div class="mb-1 flex items-center gap-2 text-xs text-[#999]">
					<span class="font-medium text-[#ccc]">{node.comment.creator_name}</span>
					<span class="flex items-center gap-0.5">
						<ArrowUp class="h-3 w-3 text-green-500" />
						{node.comment.upvotes}
					</span>
					<span class="flex items-center gap-0.5">
						<ArrowDown class="h-3 w-3 text-red-500" />
						{node.comment.downvotes}
					</span>
					<span>{formatScore(node.comment.score)} pts</span>
				</div>
				<p class="text-sm text-[#e0e0e0] leading-relaxed">{node.comment.content}</p>
			</div>
			{#if node.children.length > 0}
				{@render commentNode(node.children, depth + 1)}
			{/if}
		</div>
	{/each}
{/snippet}

<div class="max-h-[400px] overflow-y-auto">
	{@render commentNode(tree, 0)}
</div>
