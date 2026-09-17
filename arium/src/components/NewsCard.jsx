import { Link } from 'react-router-dom'

const TAG_LABELS = {
  devops: 'DevOps',
  sre: 'SRE',
  gitops: 'GitOps',
  devsecops: 'DevSecOps',
}

export default function NewsCard({ item, bookmarked, onToggleBookmark }) {
  return (
    <article className="flex flex-col gap-3 rounded-lg border border-border bg-surface p-5">
      <div className="flex flex-wrap gap-2">
        {item.tags.map((tag) => (
          <span
            key={tag}
            className="rounded-full border border-accent-border bg-accent-bg px-2.5 py-0.5 font-mono text-xs text-accent"
          >
            #{TAG_LABELS[tag] ?? tag}
          </span>
        ))}
      </div>

      <h3 className="text-lg leading-snug">
        <a
          href={item.url}
          target="_blank"
          rel="noreferrer"
          className="hover:text-accent"
        >
          {item.title}
        </a>
      </h3>

      <p className="text-sm text-text">{item.summary}</p>

      <div className="mt-auto flex items-center justify-between pt-2 font-mono text-xs text-text">
        <span>{item.source}</span>

        <div className="flex items-center gap-4">
          <Link to="/forum" className="hover:text-accent">
            Discuss on forum
          </Link>
          <button
            type="button"
            onClick={() => onToggleBookmark(item.id)}
            aria-pressed={bookmarked}
            className={bookmarked ? 'text-accent' : 'hover:text-accent'}
          >
            {bookmarked ? '★ saved' : '☆ save'}
          </button>
        </div>
      </div>
    </article>
  )
}
