import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// Ports follow the repo's 49xx scheme (see CLAUDE.md). /api goes to the API
// through this proxy so the session cookie stays same-origin.
export default defineConfig({
  plugins: [svelte()],
  server: {
    host: '127.0.0.1',
    port: 4930,
    strictPort: true,
    proxy: { '/api': 'http://127.0.0.1:4910' },
  },
})
