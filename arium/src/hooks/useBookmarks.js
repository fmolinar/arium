import { useCallback, useEffect, useState } from 'react'

const STORAGE_KEY = 'arium-bookmarks'

function readStoredIds() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function useBookmarks() {
  const [ids, setIds] = useState(() => new Set(readStoredIds()))

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
    } catch {
      // localStorage unavailable - bookmarks just won't persist across reloads
    }
  }, [ids])

  const toggle = useCallback((id) => {
    setIds((current) => {
      const next = new Set(current)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  const isBookmarked = useCallback((id) => ids.has(id), [ids])

  return { isBookmarked, toggle }
}
