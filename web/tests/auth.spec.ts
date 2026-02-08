import { test, expect, type Page } from '@playwright/test';

const ADMIN_NAME = 'Test Admin';
const ADMIN_EMAIL = 'admin@test.com';
const ADMIN_PASSWORD = 'testpassword123';

// Helper: log in and verify session is active
async function loginAndVerify(page: Page) {
	await page.goto('/login');
	await page.locator('#email').fill(ADMIN_EMAIL);
	await page.locator('#password').fill(ADMIN_PASSWORD);
	await page.getByRole('button', { name: 'Sign In' }).click();
	await expect(page).toHaveURL('/', { timeout: 10000 });
	// The client-side goto('/') after login doesn't re-fetch layout server data,
	// so we reload to ensure the session cookie is sent and session is picked up.
	await page.reload();
	await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible({ timeout: 5000 });
}

// All tests run serially since they share a single server + database.
// The database is cleaned by the webServer command before the server starts.
test.describe.configure({ mode: 'serial' });

// ============================================================
// Group 1: Forced Setup Redirect (no admin exists yet)
// ============================================================

test.describe('Forced Setup Redirect', () => {
	test('navigating to / redirects to /setup when no admin exists', async ({ page }) => {
		await page.goto('/');
		await expect(page).toHaveURL('/setup');
	});

	test('navigating to /stats redirects to /setup when no admin exists', async ({ page }) => {
		await page.goto('/stats');
		await expect(page).toHaveURL('/setup');
	});

	test('navigating to /settings redirects to /setup when no admin exists', async ({ page }) => {
		await page.goto('/settings');
		await expect(page).toHaveURL('/setup');
	});
});

// ============================================================
// Group 2: Setup Page (still no admin)
// ============================================================

test.describe('Setup Page', () => {
	test('renders correctly with all form elements', async ({ page }) => {
		await page.goto('/setup');

		await expect(page.getByRole('heading', { name: 'Initial Setup' })).toBeVisible();
		await expect(page.getByText('Create the admin account for managing settings.')).toBeVisible();
		await expect(page.locator('#name')).toBeVisible();
		await expect(page.locator('#email')).toBeVisible();
		await expect(page.locator('#password')).toBeVisible();
		await expect(page.locator('#confirmPassword')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Create Admin Account' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Sign in' })).toBeVisible();
	});

	test('shows error when passwords do not match', async ({ page }) => {
		await page.goto('/setup');

		await page.locator('#name').fill('Admin');
		await page.locator('#email').fill('admin@test.com');
		await page.locator('#password').fill('password123');
		await page.locator('#confirmPassword').fill('differentpassword');

		await page.getByRole('button', { name: 'Create Admin Account' }).click();

		await expect(page.getByText('Passwords do not match.')).toBeVisible();
	});

	test('shows error when password is too short', async ({ page }) => {
		await page.goto('/setup');

		await page.locator('#name').fill('Admin');
		await page.locator('#email').fill('admin@test.com');
		// Bypass HTML5 minlength validation by setting values directly via JS
		await page.locator('#password').evaluate((el: HTMLInputElement) => {
			el.value = 'short';
			el.dispatchEvent(new Event('input', { bubbles: true }));
		});
		await page.locator('#confirmPassword').evaluate((el: HTMLInputElement) => {
			el.value = 'short';
			el.dispatchEvent(new Event('input', { bubbles: true }));
		});

		// Submit programmatically to bypass HTML5 validation
		await page.evaluate(() => {
			const form = document.querySelector('form');
			if (form) {
				form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }));
			}
		});

		await expect(page.getByText('Password must be at least 8 characters.')).toBeVisible();
	});

	test('successfully creates admin account', async ({ page }) => {
		await page.goto('/setup');

		await page.locator('#name').fill(ADMIN_NAME);
		await page.locator('#email').fill(ADMIN_EMAIL);
		await page.locator('#password').fill(ADMIN_PASSWORD);
		await page.locator('#confirmPassword').fill(ADMIN_PASSWORD);

		await page.getByRole('button', { name: 'Create Admin Account' }).click();

		await expect(page).toHaveURL('/login', { timeout: 15000 });
	});

	// This test must run after the admin account is created above
	test('redirects to /login after admin exists', async ({ page }) => {
		await page.goto('/setup');
		await expect(page).toHaveURL('/login');
	});
});

// ============================================================
// Group 3: Login Flow (admin exists from Group 2)
// ============================================================

test.describe('Login Flow', () => {
	test('login page renders correctly', async ({ page }) => {
		await page.goto('/login');

		await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible();
		await expect(page.locator('#email')).toBeVisible();
		await expect(page.locator('#password')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
		await expect(page.getByRole('link', { name: 'Create admin account' })).toBeVisible();
	});

	test('shows error with invalid credentials', async ({ page }) => {
		await page.goto('/login');

		await page.locator('#email').fill('wrong@test.com');
		await page.locator('#password').fill('wrongpassword');
		await page.getByRole('button', { name: 'Sign In' }).click();

		// Wait for error message to appear
		const errorDiv = page.locator('.text-\\[\\#ef4444\\]');
		await expect(errorDiv).toBeVisible({ timeout: 10000 });
	});

	test('successfully logs in with valid credentials', async ({ page }) => {
		await loginAndVerify(page);
	});
});

// ============================================================
// Group 4: Protected Routes (admin exists from Group 2)
// ============================================================

test.describe('Protected Routes', () => {
	test('/settings redirects to /login when not logged in', async ({ page }) => {
		await page.goto('/settings');
		await expect(page).toHaveURL('/login');
	});

	test('/settings accessible when logged in', async ({ page }) => {
		await loginAndVerify(page);

		// Now access settings
		await page.goto('/settings');
		await expect(page).toHaveURL('/settings');
	});
});

// ============================================================
// Group 5: Logout (admin exists from Group 2)
// ============================================================

test.describe('Logout', () => {
	test('logout removes session and shows Login link', async ({ page }) => {
		await loginAndVerify(page);

		// Click logout
		await page.getByRole('button', { name: 'Logout' }).click();

		// After logout, Settings should disappear and Login should appear
		await expect(page.getByRole('link', { name: 'Login' })).toBeVisible({ timeout: 10000 });
		await expect(page.getByRole('link', { name: 'Settings' })).not.toBeVisible();
	});
});

// ============================================================
// Group 6: Complete E2E Flow
// Since the admin already exists from earlier tests, we verify the post-setup flow.
// ============================================================

test.describe('Complete End-to-End Flow', () => {
	test('full auth lifecycle: login → settings → logout → re-login', async ({ page }) => {
		// 1. With admin existing, /setup should redirect to /login
		await page.goto('/setup');
		await expect(page).toHaveURL('/login');

		// 2. Login with the credentials
		await page.locator('#email').fill(ADMIN_EMAIL);
		await page.locator('#password').fill(ADMIN_PASSWORD);
		await page.getByRole('button', { name: 'Sign In' }).click();
		await expect(page).toHaveURL('/', { timeout: 10000 });
		await page.reload();

		// 3. Verify Settings link visible
		await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible({ timeout: 5000 });

		// 4. Navigate to /settings
		await page.getByRole('link', { name: 'Settings' }).click();
		await expect(page).toHaveURL('/settings');

		// 5. Logout
		await page.getByRole('button', { name: 'Logout' }).click();

		// 6. Verify Login link visible, Settings hidden
		await expect(page.getByRole('link', { name: 'Login' })).toBeVisible({ timeout: 10000 });
		await expect(page.getByRole('link', { name: 'Settings' })).not.toBeVisible();

		// 7. Login again
		await page.getByRole('link', { name: 'Login' }).click();
		await expect(page).toHaveURL('/login');
		await page.locator('#email').fill(ADMIN_EMAIL);
		await page.locator('#password').fill(ADMIN_PASSWORD);
		await page.getByRole('button', { name: 'Sign In' }).click();
		await expect(page).toHaveURL('/', { timeout: 10000 });
		await page.reload();

		// 8. Verify logged in again
		await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible({ timeout: 5000 });
	});
});
