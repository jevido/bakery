import { getConfig } from '../config.js';

export function computePort(id, slot) {
	const config = getConfig();
	const hash = Array.from(id).reduce((acc, char) => acc + char.charCodeAt(0), 0);
	const base = config.blueGreenBasePort + (hash % 1000) * 4;
	return slot === 'blue' ? base : base + 1;
}

export function computeSlot(deployment) {
	const active = deployment.active_slot || 'blue';
	if (!deployment.blue_green_enabled) {
		return { slot: 'blue', port: computePort(deployment.id, 'blue') };
	}
	const slot = active === 'blue' ? 'green' : 'blue';
	return { slot, port: computePort(deployment.id, slot) };
}

export function resolveRuntimeArgs({ hasBuildOutput, hasStartScript }) {
	if (hasBuildOutput) {
		return ['run', 'build/index.js'];
	}
	if (hasStartScript) {
		return ['run', 'start'];
	}
	throw new Error(
		'No production entry point found. Add a build script that outputs build/index.js (recommended) or define a start script.'
	);
}

export function ensureControllableSlot(deployment) {
	if (!deployment.active_slot) {
		throw new Error('Deploy this app at least once before using start/stop controls.');
	}
	return deployment.active_slot;
}
