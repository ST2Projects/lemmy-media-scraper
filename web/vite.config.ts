import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': 'http://localhost:8081',
			'/media': 'http://localhost:8081',
			'/thumbnails': 'http://localhost:8081',
			'/ws': {
				target: 'ws://localhost:8081',
				ws: true
			}
		}
	}
});
