import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const ignoredPaths = ['**/.data/**'];

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		watch: {
			ignored: ignoredPaths
		}
	},
	preview: {
		watch: {
			ignored: ignoredPaths
		}
	}
});
