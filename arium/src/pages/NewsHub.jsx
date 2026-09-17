import { useEffect, useMemo, useRef, useState } from 'react'
import NewsCard from '../components/NewsCard'
import { useBookmarks } from '../hooks/useBookmarks'
import { MOCK_NEWS, TOPICS } from '../data/mockNews'

const PAGE_SIZE = 6

const allSorted = [...MOCK_NEWS].sort(
  (a, b) => new Date(b.publishedAt) - new Date(a.publishedAt),
)

export default function NewsHub() {
  const [activeTag, setActiveTag] = useState(null)
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)
  const [prevTag, setPrevTag] = useState(activeTag)
  const sentinelRef = useRef(null)
  const { isBookmarked, toggle } = useBookmarks()

  const filtered = useMemo(
    () => (activeTag ? allSorted.filter((item) => item.tags.includes(activeTag)) : allSorted),
    [activeTag],
  )

  // Reset pagination when the topic filter changes (adjusting state during
  // render, per https://react.dev/learn/you-might-not-need-an-effect).
  if (activeTag !== prevTag) {
    setPrevTag(activeTag)
    setVisibleCount(PAGE_SIZE)
  }

  const visible = filtered.slice(0, visibleCount)
  const hasMore = visibleCount < filtered.length

  useEffect(() => {
    const node = sentinelRef.current
    if (!node || !hasMore) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          setVisibleCount((count) => count + PAGE_SIZE)
        }
      },
      { rootMargin: '200px' },
    )

    observer.observe(node)
    return () => observer.disconnect()
  }, [hasMore])

  return (
    <section className="mx-auto max-w-4xl px-6 py-14">
      <p className="mb-2 font-mono text-sm text-accent">[ NEWS ]</p>
      <h1 className="text-3xl">News Hub</h1>

      <div className="mt-6 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => setActiveTag(null)}
          className={`rounded-full border px-3 py-1 font-mono text-xs transition-colors ${
            activeTag === null
              ? 'border-accent bg-accent-bg text-accent'
              : 'border-border text-text hover:border-accent hover:text-accent'
          }`}
        >
          All
        </button>
        {TOPICS.map((topic) => (
          <button
            key={topic.slug}
            type="button"
            onClick={() => setActiveTag(topic.slug)}
            className={`rounded-full border px-3 py-1 font-mono text-xs transition-colors ${
              activeTag === topic.slug
                ? 'border-accent bg-accent-bg text-accent'
                : 'border-border text-text hover:border-accent hover:text-accent'
            }`}
          >
            #{topic.label}
          </button>
        ))}
      </div>

      <div className="mt-8 flex flex-col gap-4">
        {visible.map((item) => (
          <NewsCard
            key={item.id}
            item={item}
            bookmarked={isBookmarked(item.id)}
            onToggleBookmark={toggle}
          />
        ))}

        {visible.length === 0 && (
          <p className="py-12 text-center font-mono text-sm text-text">
            No stories for this topic yet.
          </p>
        )}
      </div>

      {hasMore && (
        <div ref={sentinelRef} className="py-8 text-center font-mono text-xs text-text">
          loading more…
        </div>
      )}
    </section>
  )
}
