import { constants } from 'node:fs';
import { access } from 'node:fs/promises';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';

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
		'--nginx',
		'--keep-until-expiring',
		'--expand',
		'--agree-tos',
		'--non-interactive',
		'--email',
		config.certbotEmail,
		'--cert-name',
		domains[0]
	];

	domains.forEach((domain) => {
		args.push('-d', domain);
	});

	await log('info', 'Requesting TLS certificate', { domains });
	const process = spawn(['certbot', ...args], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stdout = await new Response(process.stdout).text();
	const stderr = await new Response(process.stderr).text();

	if (process.exitCode !== 0) {
		await log('error', 'Certbot failed', { stderr });
		throw new Error(stderr.trim() || 'Certbot failed');
	}
	await log('info', 'Certbot completed', { stdout });
}
