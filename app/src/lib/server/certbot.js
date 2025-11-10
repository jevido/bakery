import { constants } from 'node:fs';
import { access } from 'node:fs/promises';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';
import { stopService, startService } from './systemd.js';

const PATH_FALLBACK = '/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin';

export async function certificateExists(domain) {
	if (!domain) return false;
	const base = `/etc/letsencrypt/live/${domain}`;
	try {
		await Promise.all([
			access(`${base}/fullchain.pem`, constants.R_OK),
			access(`${base}/privkey.pem`, constants.R_OK)
		]);
		return true;
	} catch {
		return false;
	}
}

export async function requestCertificate(domains) {
	const config = getConfig();
	if (!domains.length) return;
	if (!config.certbotEmail) {
		throw new Error('CERTBOT_EMAIL is not configured. Cannot request certificates.');
	}

	const args = [
		'certonly',
		'--standalone',
		'--keep-until-expiring',
		'--expand',
		'--agree-tos',
		'--non-interactive',
		'--email',
		config.certbotEmail,
		'--cert-name',
		domains[0]
	];

	for (const domain of domains) {
		args.push('-d', domain);
	}

	await log('info', 'Requesting TLS certificate', { domains, mode: 'standalone' });
	await stopService('nginx').catch((error) => {
		log('error', 'Failed to stop nginx before certbot', { error: error.message });
		throw new Error('Unable to stop nginx before requesting certificate');
	});
	let needsRestart = true;
	const envPath = process.env.PATH ? `${process.env.PATH}:${PATH_FALLBACK}` : PATH_FALLBACK;
	const child = spawn(['certbot', ...args], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe',
		env: {
			...process.env,
			PATH: envPath
		}
	});
	const stdout = await new Response(child.stdout).text();
	const stderr = await new Response(child.stderr).text();
	const exitCode = await child.exited;

	if (exitCode !== 0) {
		if (needsRestart) {
			await startService('nginx').catch((error) =>
				log('error', 'Failed to restart nginx after certbot failure', {
					error: error.message
				})
			);
		}
		await log('error', 'Certbot failed', { stderr });
		throw new Error(stderr.trim() || 'Certbot failed');
	}
	if (needsRestart) {
		await startService('nginx');
		await log('info', 'Nginx restarted after certbot');
	}
	await log('info', 'Certbot completed', { stdout });
}
