// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
import type { auth } from '$lib/server/auth';

declare global {
	namespace App {
		// interface Error {}
		interface Locals {
			session: Awaited<ReturnType<typeof auth.api.getSession>> | null;
		}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
