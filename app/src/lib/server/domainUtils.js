import { getConfig } from './config.js';
import { isLocalHostname } from '$lib/shared/domainRules.js';

export { isLocalHostname };

export function shouldSkipTls(domains = [], config = getConfig()) {
	if (config.localMode) {
		return true;
	}
	if (!Array.isArray(domains) || domains.length === 0) {
		return false;
	}
	return domains.every((domain) => isLocalHostname(domain.hostname));
}

function resolveBaseHost(config) {
	try {
		const url = new URL(config.baseUrl);
		return url.hostname;
	} catch {
		return null;
	}
}

export function getLocalResolutionTarget(config = getConfig()) {
	if (config.localMode) {
		return '127.0.0.1';
	}
	return config.publicIp || resolveBaseHost(config) || config.host || '127.0.0.1';
}

export function getLocalResolutionHint(hostname, config = getConfig()) {
	if (!isLocalHostname(hostname)) {
		return null;
	}
	const target = getLocalResolutionTarget(config);
	return {
		target,
		instructions: `Add \"${target} ${hostname}\" to your /etc/hosts (macOS/Linux) or C:\\\\Windows\\\\System32\\\\drivers\\\\etc\\\\hosts (Windows) so this domain resolves to the Bakery control plane.`,
		localOnly: true
	};
}
