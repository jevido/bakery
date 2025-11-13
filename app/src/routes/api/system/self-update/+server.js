import { error, json } from '@sveltejs/kit';
import { getConfig } from '$lib/server/config.js';
import {
	getSelfUpdateStatus,
	serializeSelfUpdateStatus
} from '$lib/server/selfUpdater.js';

export const GET = async ({ locals }) => {
	if (!locals.user?.is_admin) {
		throw error(403, 'Unauthorized');
	}

	const config = getConfig();
	const status = serializeSelfUpdateStatus(getSelfUpdateStatus());

	return json({
		status,
		config: {
			repo: config.selfUpdateRepository,
			branch: config.selfUpdateBranch,
			webhookConfigured: Boolean(config.githubAppWebhookSecret)
		}
	});
};
