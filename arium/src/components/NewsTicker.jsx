import { Link } from 'react-router-dom'
import { MOCK_NEWS } from '../data/mockNews'

const headlines = [...MOCK_NEWS]
  .sort((a, b) => new Date(b.publishedAt) - new Date(a.publishedAt))
  .slice(0, 5)

export default function NewsTicker() {
  // Duplicate the list so the CSS animation can loop seamlessly at -50%.
  const track = [...headlines, ...headlines]

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-[var(--shadow)]">
      <div className="flex items-center gap-2 border-b border-border px-4 py-2">
        <span className="h-2.5 w-2.5 rounded-full bg-red-400/70" />
        <span className="h-2.5 w-2.5 rounded-full bg-yellow-400/70" />
        <span className="h-2.5 w-2.5 rounded-full bg-green-400/70" />
        <span className="ml-2 font-mono text-xs text-text">breaking-news.feed</span>
      </div>

      <div className="group relative flex overflow-hidden py-3">
        <ul className="flex shrink-0 animate-ticker items-center gap-10 pr-10 group-hover:[animation-play-state:paused]">
          {track.map((item, i) => (
            <li key={`${item.id}-${i}`} className="shrink-0">
              <Link
                to="/news"
                className="flex items-center gap-2 font-mono text-sm text-text hover:text-accent"
              >
                <span className="text-accent">&gt;</span>
                {item.title}
                <span className="text-text/60">— {item.source}</span>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
