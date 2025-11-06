import { error } from '@sveltejs/kit';
import { authenticateAgent } from './models/nodeModel.js';

export async function requireAgent(request, { allowInactive = false } = {}) {
	const header = request.headers.get('authorization') || '';
	const [, token] = header.split(' ');
	if (!token) {
		throw error(401, 'Missing authorization token');
	}
	const node = await authenticateAgent(token.trim());
	if (!node) {
		throw error(401, 'Invalid agent token');
	}
	if (!allowInactive && node.status !== 'active') {
		throw error(403, 'Node is not active');
	}
	return node;
}
