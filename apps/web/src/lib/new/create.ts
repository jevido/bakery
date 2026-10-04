import { ApiError } from '../api'
import { toast } from '../ui/toast.svelte'

/**
 * Runs a create request for one of the create pages. A refusal with errors
 * for fields the page shows goes under those fields; any other refusal
 * becomes an error toast, as Coolify's handleError() does.
 */
export async function attempt<T>(
  run: () => Promise<T>,
  fields: string[],
  onerrors: (errors: Record<string, string>) => void,
): Promise<T | undefined> {
  onerrors({})
  try {
    return await run()
  } catch (err) {
    if (!(err instanceof ApiError)) throw err
    const shown = Object.keys(err.errors).filter((k) => fields.includes(k))
    if (shown.length > 0) onerrors(err.errors)
    else toast.error(err.message)
    return undefined
  }
}
