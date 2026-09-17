import { Link } from 'react-router-dom'
import NewsTicker from '../components/NewsTicker'

export default function Home() {
  return (
    <section className="mx-auto max-w-6xl px-6 py-20">
      <p className="mb-4 font-mono text-sm text-accent">[ ARIUM ]</p>

      <h1 className="max-w-3xl text-4xl leading-tight sm:text-5xl">
        Your unified terminal for{' '}
        <span className="text-accent">DevOps, SRE, GitOps, and DevSecOps</span> intelligence.
      </h1>

      <p className="mt-6 max-w-2xl font-mono text-base text-text">
        One feed for the news, incidents, and discussions that matter to infrastructure
        engineers — curated, tagged, and searchable.
      </p>

      <div className="mt-8 flex flex-wrap gap-4">
        <Link
          to="/news"
          className="rounded border border-accent bg-accent-bg px-5 py-2.5 font-mono text-sm text-accent transition-colors hover:bg-accent hover:text-bg"
        >
          Explore full news →
        </Link>
        <Link
          to="/forum"
          className="rounded border border-border px-5 py-2.5 font-mono text-sm text-text transition-colors hover:border-accent hover:text-accent"
        >
          Join the forum →
        </Link>
      </div>

      <div className="mt-16">
        <NewsTicker />
      </div>
    </section>
  )
}
