import { serviceStatus, serviceNameForDeployment } from './systemd.js';

export async function getRuntimeStatus(deployment) {
	if (!deployment?.active_slot) {
		return {
			state: 'inactive',
			reason: 'no_active_slot'
		};
	}

	if (deployment.node_id) {
		return {
			state: 'unknown',
			reason: 'remote_node'
		};
	}

	const serviceName = serviceNameForDeployment(deployment.id, deployment.active_slot);
	const serviceUnit = `${serviceName}.service`;

	try {
		const status = await serviceStatus(serviceUnit);
		if (status === 'active') {
			return {
				state: 'running',
				service: serviceUnit
			};
		}
		if (status === 'inactive') {
			return {
				state: 'stopped',
				service: serviceUnit
			};
		}
		return {
			state: 'unknown',
			reason: 'status_unavailable',
			service: serviceUnit
		};
	} catch (error) {
		return {
			state: 'unknown',
			reason: 'status_check_failed',
			error: error?.message,
			service: serviceUnit
		};
	}
}
