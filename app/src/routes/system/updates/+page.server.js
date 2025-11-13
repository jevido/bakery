import { error } from '@sveltejs/kit';
import { getConfig } from '$lib/server/config.js';
import {
	getSelfUpdateStatus,
	serializeSelfUpdateStatus
} from '$lib/server/selfUpdater.js';

export async function load({ locals }) {
	if (!locals.user?.is_admin) {
		throw error(403, 'Forbidden');
	}

	const config = getConfig();
	const status = serializeSelfUpdateStatus(getSelfUpdateStatus());

	return {
		selfUpdate: {
			status,
			config: {
				repo: config.selfUpdateRepository,
				branch: config.selfUpdateBranch,
				webhookConfigured: Boolean(config.githubAppWebhookSecret)
			}
		}
	};
}
