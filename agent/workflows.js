import { mkdir, rm, writeFile, access, readdir, unlink } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { nanoid } from 'nanoid';
import { getConfig } from '../app/src/lib/server/config.js';
import { writeDeploymentConfig, reloadNginx } from '../app/src/lib/server/nginx.js';
import { requestCertificate } from '../app/src/lib/server/certbot.js';
import {
	stopService,
	startService,
	reloadDaemon,
	enableService,
	serviceNameForDeployment
} from '../app/src/lib/server/systemd.js';
import {
	detectDockerfile,
	buildImage,
	runContainer,
	stopAndRemoveContainer
} from '../app/src/lib/server/docker.js';
import { createLogger } from '../app/src/lib/server/logger.js';
import {
	logDeployment,
	updateDeploymentStatus,
	recordDeploymentVersionRemote,
	getDeploymentContext
} from './apiClient.js';

const logger = createLogger('agent:workflows');

function computeSlot(deployment) {
	const active = deployment.active_slot || 'blue';
	if (!deployment.blue_green_enabled) {
		return { slot: 'blue', port: computePort(deployment.id, 'blue') };
	}
	const slot = active === 'blue' ? 'green' : 'blue';
	return { slot, port: computePort(deployment.id, slot) };
}

function computePort(id, slot) {
	const config = getConfig();
	const hash = Array.from(id).reduce((acc, char) => acc + char.charCodeAt(0), 0);
	const base = config.blueGreenBasePort + (hash % 1000) * 4;
	return slot === 'blue' ? base : base + 1;
}

async function runCommand(command, args, options = {}) {
	await logger.info('Running command', { command, args });
	const process = spawn([command, ...args], {
		cwd: options.cwd,
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stdout = await new Response(process.stdout).text();
	const stderr = await new Response(process.stderr).text();

	if (process.exitCode !== 0) {
		await logger.error('Command failed', { command, args, stderr });
		throw new Error(stderr || `Command ${command} failed`);
	}
	return stdout;
}

async function cloneRepository({ repository, branch, accessToken, targetDir }) {
	const [owner, repo] = repository.split('/');
	const tokenUrl = accessToken
		? `https://${accessToken}@github.com/${owner}/${repo}.git`
		: `https://github.com/${owner}/${repo}.git`;
	await runCommand('git', ['clone', '--depth', '1', '--branch', branch, tokenUrl, targetDir]);
}

async function installDependencies(dir) {
	await runCommand('bun', ['install'], { cwd: dir });
}

async function buildProject(dir) {
	try {
		await access(join(dir, 'bunfig.toml'));
		await runCommand('bun', ['run', 'build'], { cwd: dir });
		return join(dir, 'build');
	} catch {
		await logger.info('No build script detected, skipping build');
		return dir;
	}
}

async function createSystemdUnit({ deployment, slot, port, workingDir, env }) {
	const config = getConfig();
	const templatePath = join(
		process.cwd(),
		'infrastructure',
		'systemd',
		'deployments',
		'service.template'
	);
	const template = await Bun.file(templatePath).text();
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	const ExecStart = deployment.dockerized
		? `/usr/bin/docker start -a ${serviceName}`
		: `/usr/bin/env PORT=${port} bun run start`;

	const envLines = Object.entries(env)
		.map(([key, value]) => `Environment="${key}=${String(value).replace(/"/g, '\\"')}"`)
		.join('\n');

	const rendered = template
		.replace('{{SERVICE_NAME}}', serviceName)
		.replace('{{WORKING_DIRECTORY}}', workingDir)
		.replace('{{EXEC_START}}', ExecStart)
		.replace('{{ENVIRONMENT}}', envLines);

	const targetPath = join(config.systemdServicesDir, `${serviceName}.service`);
	await writeFile(targetPath, rendered, 'utf8');
	await reloadDaemon();
	await enableService(`${serviceName}.service`);
	return targetPath;
}

async function deployDockerApp({ deployment, slot, port, repoDir, env }) {
	const imageTag = `bakery/${deployment.id}:${slot}`;
	const serviceName = serviceNameForDeployment(deployment.id, slot);
	await stopAndRemoveContainer(serviceName);
	await buildImage({ context: repoDir, tag: imageTag });
	await runContainer({
		image: imageTag,
		name: serviceName,
		env: {
			...env,
			PORT: String(port)
		},
		portMapping: { host: port, container: env.PORT || 3000 }
	});
}

async function deployBunApp({ deployment, slot, port, repoDir, env }) {
	await createSystemdUnit({
		deployment,
		slot,
		port,
		workingDir: repoDir,
		env: {
			...env,
			PORT: String(port)
		}
	});
	try {
		await stopService(`${serviceNameForDeployment(deployment.id, slot)}.service`);
	} catch {}
	await startService(`${serviceNameForDeployment(deployment.id, slot)}.service`);
}

async function cleanupOldBuilds(deployment) {
	const config = getConfig();
	const dir = join(config.buildsDir, deployment.id);
	const keep = config.releasesToKeep;
	const entries = await readdir(dir).catch(() => []);
	if (entries.length > keep) {
		const sorted = entries.sort();
		const toRemove = sorted.slice(0, entries.length - keep);
		await Promise.all(
			toRemove.map((entry) => rm(join(dir, entry), { recursive: true, force: true }))
		);
		await logger.info('Cleaned up builds', { deploymentId: deployment.id, removed: toRemove });
	}
}

export async function deployTask({ deploymentId, commitSha }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment, environment, domains, githubAccessToken } = context;
	const { slot, port } = computeSlot(deployment);
	const buildId = nanoid();
	const config = getConfig();
	const deploymentDir = join(config.buildsDir, deployment.id);
	const slotDir = join(deploymentDir, `${slot}-${buildId}`);
	await mkdir(slotDir, { recursive: true });
	const repoDir = join(slotDir, 'source');
	await mkdir(repoDir, { recursive: true });

	const envVars = {
		...environment,
		PORT: String(port)
	};

	await updateDeploymentStatus(deployment.id, { status: 'deploying' });
	await logDeployment(deployment.id, 'info', 'Cloning repository', {});

	await cloneRepository({
		repository: deployment.repository,
		branch: deployment.branch,
		accessToken: githubAccessToken,
		targetDir: repoDir
	});

	const dockerized = await detectDockerfile(repoDir);
	await logDeployment(deployment.id, 'info', 'Detected project type', { dockerized });
	deployment.dockerized = dockerized;

	if (!dockerized) {
		await installDependencies(repoDir);
		await buildProject(repoDir);
	}

	if (dockerized) {
		await deployDockerApp({ deployment, slot, port, repoDir, env: envVars });
	} else {
		await deployBunApp({ deployment, slot, port, repoDir, env: envVars });
	}

	if (domains.length) {
		await writeDeploymentConfig({
			deployment,
			domains,
			port,
			slot
		});
		await requestCertificate(domains.map((d) => d.hostname));
		await reloadNginx();
	}

	const version = await recordDeploymentVersionRemote(deployment.id, {
		slot,
		commitSha,
		status: 'active',
		port,
		dockerized,
		artifactPath: repoDir
	});

	await updateDeploymentStatus(deployment.id, {
		status: 'running',
		active_slot: slot,
		dockerized
	});

	await logDeployment(deployment.id, 'info', 'Deployment completed', {
		slot,
		port,
		versionId: version?.versionId || version
	});

	await cleanupOldBuilds(deployment);

	return { slot, port };
}

export async function activateVersionTask({ deploymentId, version }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment, domains } = context;
	await writeDeploymentConfig({
		deployment,
		domains,
		port: version.port,
		slot: version.slot
	});
	await reloadNginx();
	await updateDeploymentStatus(deployment.id, {
		active_slot: version.slot,
		status: 'running'
	});
	await logDeployment(deployment.id, 'info', 'Activated deployment version', {
		versionId: version.id,
		slot: version.slot
	});
}

export async function cleanupDeploymentTask({ deploymentId }) {
	const context = await getDeploymentContext(deploymentId);
	const { deployment } = context;
	const config = getConfig();
	const slots = deployment.blue_green_enabled ? ['blue', 'green'] : ['blue'];
	for (const slot of slots) {
		const serviceName = serviceNameForDeployment(deployment.id, slot);
		try {
			await stopService(`${serviceName}.service`);
		} catch {}
		try {
			await stopAndRemoveContainer(serviceName);
		} catch {}
		const servicePath = join(config.systemdServicesDir, `${serviceName}.service`);
		await unlink(servicePath).catch(() => {});
	}
	await reloadDaemon().catch(() => {});
	const buildDir = join(config.buildsDir, deployment.id);
	await rm(buildDir, { recursive: true, force: true }).catch(() => {});
	const nginxConfig = join(config.nginxSitesDir, `${deployment.id}.conf`);
	await unlink(nginxConfig).catch(() => {});
	await reloadNginx().catch(() => {});
	await logDeployment(deployment.id, 'info', 'Cleaned up deployment resources', {});
	await updateDeploymentStatus(deployment.id, { status: 'deleted' }).catch(() => {});
}
