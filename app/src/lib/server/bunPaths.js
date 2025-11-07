export function getBunExecutable() {
	const execPath = process.execPath || '';
	if (execPath.toLowerCase().includes('bun')) {
		return execPath;
	}
	if (process.env.BUN_EXECUTABLE) {
		return process.env.BUN_EXECUTABLE;
	}
	return 'bun';
}
