import { Link } from 'react-router-dom'
import { CONTACT_URL, EFFECTIVE_DATE } from '../legal/config'

// Shared layout for the Terms of Service and Privacy Policy: a header with the
// effective date and the other document's link, then numbered sections.
export default function LegalPage({ label, title, intro, sections, related }) {
  return (
    <section className="mx-auto max-w-2xl px-6 py-14">
      <p className="mb-2 font-mono text-sm text-accent">[ {label} ]</p>
      <h1 className="text-3xl">{title}</h1>
      <p className="mt-3 font-mono text-xs text-text">Effective {EFFECTIVE_DATE}</p>

      <div className="mt-8 flex flex-col gap-8 font-mono text-sm leading-relaxed text-text">
        <p>{intro}</p>

        {sections.map(({ id, heading, body }, i) => (
          <div key={id} id={id} className="scroll-mt-6">
            <h2 className="mb-2 text-base text-text-h">
              {i + 1}. {heading}
            </h2>
            <div className="flex flex-col gap-3">{body}</div>
          </div>
        ))}

        <p className="border-t border-border pt-6 text-xs">
          Questions about this page? Open an issue on{' '}
          <ExternalLink href={CONTACT_URL}>GitHub</ExternalLink>. See also the{' '}
          <Link to={related.to} className="text-accent hover:underline">
            {related.label}
          </Link>
          .
        </p>
      </div>
    </section>
  )
}

export function ExternalLink({ href, children }) {
  return (
    <a href={href} target="_blank" rel="noreferrer" className="text-accent hover:underline">
      {children}
    </a>
  )
}

export function List({ items }) {
  return (
    <ul className="flex list-disc flex-col gap-1.5 pl-5">
      {items.map((item, i) => (
        <li key={i}>{item}</li>
      ))}
    </ul>
  )
}
