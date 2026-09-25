import { useCallback, useEffect, useState } from 'react'
import { fetchNews } from '../api/news'

function firstPage(tag) {
  return { tag, items: [], cursor: null, done: false, loading: true, error: null }
}

// useNews pages through GET /api/v1/news for one tag (null for all topics).
// The first page loads immediately; loadMore fetches the next one. Changing
// tag starts over from the first page.
export function useNews(tag, pageSize) {
  const [state, setState] = useState(() => firstPage(tag))

  // Reset when the tag changes (adjusting state during render, per
  // https://react.dev/learn/you-might-not-need-an-effect).
  if (state.tag !== tag) {
    setState(firstPage(tag))
  }

  const { loading, cursor } = state

  useEffect(() => {
    if (!loading) return

    const controller = new AbortController()
    fetchNews({ tag, limit: pageSize, cursor, signal: controller.signal })
      .then((page) => {
        setState((s) => ({
          ...s,
          items: [...s.items, ...page.items],
          cursor: page.nextCursor ?? null,
          done: !page.nextCursor,
          loading: false,
        }))
      })
      .catch((error) => {
        if (controller.signal.aborted) return
        setState((s) => ({ ...s, error, loading: false }))
      })

    return () => controller.abort()
  }, [tag, pageSize, cursor, loading])

  const loadMore = useCallback(() => {
    setState((s) => (s.loading || s.done || s.error ? s : { ...s, loading: true }))
  }, [])

  const retry = useCallback(() => {
    setState((s) => (s.error ? { ...s, error: null, loading: true } : s))
  }, [])

  return {
    items: state.items,
    loading: state.loading,
    error: state.error,
    hasMore: !state.done,
    loadMore,
    retry,
  }
}
