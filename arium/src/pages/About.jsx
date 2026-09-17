export default function About() {
  return (
    <section className="mx-auto max-w-2xl px-6 py-14">
      <p className="mb-2 font-mono text-sm text-accent">[ ABOUT ]</p>
      <h1 className="text-3xl">About arium</h1>

      <div className="mt-8 flex flex-col gap-8 font-mono text-sm leading-relaxed text-text">
        <div>
          <h2 className="mb-2 text-base text-text-h">Mission</h2>
          <p>
            arium is one feed for the news, incidents, and discussions that matter to
            infrastructure engineers — DevOps, SRE, GitOps, and DevSecOps in a single place,
            instead of a dozen scattered blogs, subreddits, and mailing lists.
          </p>
        </div>

        <div>
          <h2 className="mb-2 text-base text-text-h">How the data works</h2>
          <p>
            News Hub currently runs on a small set of curated placeholder stories while the
            aggregation pipeline is being built. The plan is to pull from public RSS feeds and
            APIs across the four topic areas, tag and de-duplicate automatically, and surface
            the highest-signal stories first.
          </p>
        </div>

        <div>
          <h2 className="mb-2 text-base text-text-h">Open source</h2>
          <p>
            arium is a personal project built in the open to showcase full-stack engineering
            across frontend, backend, SRE, and DevOps. The source is on{' '}
            <a
              href="https://github.com/fmolinar/arium"
              target="_blank"
              rel="noreferrer"
              className="text-accent hover:underline"
            >
              GitHub
            </a>
            .
          </p>
        </div>

        <div>
          <h2 className="mb-2 text-base text-text-h">Stack</h2>
          <p>React + Tailwind on the frontend, Go + MongoDB on the backend, all containerized with Docker.</p>
        </div>
      </div>
    </section>
  )
}
