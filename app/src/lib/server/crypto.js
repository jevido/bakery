import { createCipheriv, createDecipheriv, randomBytes, createHmac } from 'node:crypto';
import { getConfig } from './config.js';

function getKey() {
	const { encryptionKey } = getConfig();
	const buffer = Buffer.from(encryptionKey, 'utf8');
	if (buffer.length !== 32) {
		return Buffer.from(createHmac('sha256', buffer).digest('hex').slice(0, 32));
	}
	return buffer;
}

export function encrypt(value) {
	const key = getKey();
	const iv = randomBytes(12);
	const cipher = createCipheriv('aes-256-gcm', key, iv);
	const encrypted = Buffer.concat([cipher.update(value, 'utf8'), cipher.final()]);
	const tag = cipher.getAuthTag();
	return Buffer.concat([iv, tag, encrypted]).toString('base64');
}

export function decrypt(payload) {
	const key = getKey();
	const buffer = Buffer.from(payload, 'base64');
	const iv = buffer.subarray(0, 12);
	const tag = buffer.subarray(12, 28);
	const ciphertext = buffer.subarray(28);
	const decipher = createDecipheriv('aes-256-gcm', key, iv);
	decipher.setAuthTag(tag);
	const decrypted = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
	return decrypted.toString('utf8');
}
