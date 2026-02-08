import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './tests',
	fullyParallel: false,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: 1,
	reporter: 'html',
	use: {
		baseURL: 'http://localhost:4567',
		trace: 'on-first-retry'
	},
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' }
		}
	],
	webServer: {
		command: 'rm -f /tmp/playwright-auth-test.db && node build/index.js',
		url: 'http://localhost:4567',
		reuseExistingServer: !process.env.CI,
		env: {
			BETTER_AUTH_SECRET: 'test-secret-that-is-at-least-thirty-two-characters-long',
			AUTH_DB_PATH: '/tmp/playwright-auth-test.db',
			ORIGIN: 'http://localhost:4567',
			PORT: '4567'
		},
		timeout: 60000
	}
});
