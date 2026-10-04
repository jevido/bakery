// What Coolify's create pages do to what a person types before they submit
// (Apache-2.0, see NOTICE).

/**
 * PublicGitRepository::loadBranch() and getGitSource() without the branch
 * lookup, which The Bakery has no Git provider API for: an scp-style URL
 * becomes https, a `/tree/<branch>` link names the branch, and the URL ends
 * in `.git` except on github.com. Throws with Coolify's message when the URL
 * is not an http(s) or scp-style Git URL.
 */
export function publicRepository(raw: string): { url: string; branch: string } {
  let value = raw.trim()
  // scpStyleGitUrlToHttps()
  const scp = /^[a-zA-Z0-9._-]+@([a-zA-Z0-9.-]+):(?!\/\/)(.+)$/.exec(value)
  if (scp) value = `https://${scp[1]}/${scp[2]}`
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    throw new Error('Invalid repository URL: The repository URL is not a valid URL.')
  }
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') {
    throw new Error('Invalid repository URL: The repository URL must start with https:// or http://.')
  }
  if (parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error('Invalid repository URL: The repository URL should not contain credentials, query parameters or fragments.')
  }
  const segments = parsed.pathname.split('/').filter(Boolean)
  if (segments.length < 2) throw new Error('Invalid repository URL: The repository URL names no owner and repository.')
  let branch = 'main'
  let path = parsed.pathname.replace(/\/+$/, '')
  if (segments[2] === 'tree' && segments.length > 3) {
    branch = segments.slice(3).join('/')
    path = `/${segments[0]}/${segments[1]}`
  }
  let url = `${parsed.protocol}//${parsed.host}${path}`
  if (parsed.hostname === 'github.com') url = url.replace(/\.git$/, '')
  else if (!url.endsWith('.git')) url += '.git'
  return { url, branch }
}

/** DockerImageParser::parse(): the name, and the tag or SHA256 digest. */
export function dockerImage(raw: string): { name: string; tag: string; digest: string } {
  const value = raw.trim()
  const hash = /^(.+)@sha256:([a-f0-9]{64})$/i.exec(value)
  if (hash) return { name: hash[1], tag: '', digest: hash[2] }
  const colon = value.lastIndexOf(':')
  const slash = value.lastIndexOf('/')
  if (colon !== -1 && colon > slash) {
    const tag = value.slice(colon + 1)
    if (/^[a-f0-9]{64}$/i.test(tag)) return { name: value.slice(0, colon), tag: '', digest: tag }
    return { name: value.slice(0, colon), tag, digest: '' }
  }
  return { name: value, tag: '', digest: '' }
}
