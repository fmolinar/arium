export const TOPICS = [
  { slug: 'devops', label: 'DevOps' },
  { slug: 'sre', label: 'SRE' },
  { slug: 'gitops', label: 'GitOps' },
  { slug: 'devsecops', label: 'DevSecOps' },
]

// Relative URL: the Vite dev/preview server proxies /api to the backend (see
// vite.config.js), so the build doesn't depend on where the API lives.
const NEWS_URL = '/api/v1/news'

// fetchNews returns one page of articles, newest first:
// { items: [...], nextCursor?: string }. Pass nextCursor back as `cursor` to
// get the following page; it's absent on the last page.
export async function fetchNews({ tag, limit, cursor, signal } = {}) {
  const params = new URLSearchParams()
  if (tag) params.set('tag', tag)
  if (limit) params.set('limit', String(limit))
  if (cursor) params.set('cursor', cursor)

  const res = await fetch(`${NEWS_URL}?${params}`, { signal })
  if (!res.ok) {
    throw new Error(`news request failed: ${res.status}`)
  }
  return res.json()
}
