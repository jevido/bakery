import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';

export async function requestCertificate(domains) {
	const config = getConfig();
	if (!domains.length) return;

	const args = [
		'certonly',
		'--nginx',
		'--agree-tos',
		'--non-interactive',
		'--email',
		config.certbotEmail
	];

	domains.forEach((domain) => {
		args.push('-d', domain);
	});

	const process = spawn(['certbot', ...args], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stdout = await new Response(process.stdout).text();
	const stderr = await new Response(process.stderr).text();

	if (process.exitCode !== 0) {
		await log('error', 'Certbot failed', { stderr });
		throw new Error(stderr);
	}
	await log('info', 'Certbot completed', { stdout });
}
