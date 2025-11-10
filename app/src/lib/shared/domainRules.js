export function isPrivateIp(hostname = '') {
	const match = hostname.trim().match(
		/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/
	);
	if (!match) return false;
	const [a, b] = match.slice(1).map(Number);
	if (a === 10 || a === 127) return true;
	if (a === 192 && b === 168) return true;
	if (a === 169 && b === 254) return true;
	if (a === 172 && b >= 16 && b <= 31) return true;
	return false;
}

export function isLocalHostname(hostname = '') {
	const normalized = hostname.trim().toLowerCase();
	if (!normalized) return false;
	if (normalized === 'localhost') return true;
	if (normalized.endsWith('.localhost') || normalized.endsWith('.local')) return true;
	if (isPrivateIp(normalized)) return true;
	return false;
}
