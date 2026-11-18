import { Link } from 'react-router-dom'
import LegalPage, { ExternalLink, List } from '../components/LegalPage'
import { CONTACT_URL, OPERATOR, SITE_NAME, SITE_URL } from '../legal/config'

const SECTIONS = [
  {
    id: 'summary',
    heading: 'The short version',
    body: (
      <List
        items={[
          'No ads, no analytics or tracking scripts, and no cookies.',
          'Our servers log basic request details (such as your IP address) to keep the site running and secure.',
          'If you create an account, we store your name, email address and a hashed password.',
          'Your theme and bookmarks stay in your own browser.',
          'We don’t sell or share your data for marketing.',
        ]}
      />
    ),
  },
  {
    id: 'who',
    heading: 'Who we are',
    body: (
      <p>
        {SITE_NAME} ({SITE_URL}) is a personal, non-commercial project run by {OPERATOR}, who is
        responsible for the personal data described here.
      </p>
    ),
  },
  {
    id: 'collected',
    heading: 'What we collect and why',
    body: (
      <>
        <p>
          <span className="text-text-h">Request logs.</span> When your browser talks to our API,
          the server logs the time, the page or endpoint requested, the response status and
          timing, your IP address, your browser&apos;s user-agent string and a random request ID.
          We use these logs to operate the site, debug problems and detect abuse. That&apos;s our
          legitimate interest in running a working, secure service.
        </p>
        <p>
          <span className="text-text-h">Service metrics.</span> We count requests, errors and
          response times per API route, and track news-collection runs. These numbers are
          aggregated and don&apos;t identify you.
        </p>
        <p>
          <span className="text-text-h">Account data.</span> If you register, we store your name,
          email address, a one-way bcrypt hash of your password (never the password itself), your
          role, and when the account was created and updated. We use them to sign you in and run
          your account. Signing in gives you a token that expires after 24 hours. We don&apos;t
          send marketing email.
        </p>
        <p>
          <span className="text-text-h">News content.</span> The news feed is built from public
          feeds about publishers&apos; articles, not from data about you.
        </p>
      </>
    ),
  },
  {
    id: 'browser-storage',
    heading: 'Cookies and browser storage',
    body: (
      <p>
        We don&apos;t set cookies. The site saves your light/dark theme choice and your bookmarked
        articles in your browser&apos;s local storage, so they&apos;re never sent to us. Clearing
        your browser&apos;s site data removes them. Fonts are served from our own server, so
        loading the site doesn&apos;t contact a font provider.
      </p>
    ),
  },
  {
    id: 'sharing',
    heading: 'Who processes your data',
    body: (
      <>
        <p>We don&apos;t sell personal data. It&apos;s handled only by services that run the site:</p>
        <List
          items={[
            'Cloudflare, which carries traffic between your browser and our server (and so sees your IP address and requests) and may filter malicious traffic;',
            'MongoDB Atlas, a hosted database that may store account data and news;',
            'the server that hosts the site, operated by us.',
          ]}
        />
        <p>
          These providers may process data in countries other than yours, including the United
          States. We may also disclose data if the law requires it.
        </p>
        <p>
          Links in the News Hub take you to other publishers&apos; sites, which have their own
          privacy policies.
        </p>
      </>
    ),
  },
  {
    id: 'retention',
    heading: 'How long we keep it',
    body: (
      <p>
        Request logs are capped in size and rotated automatically, so older entries are
        overwritten as new traffic arrives. Account data is kept until you ask us to delete your
        account. Aggregated metrics contain no personal data and expire after a few weeks.
      </p>
    ),
  },
  {
    id: 'rights',
    heading: 'Your rights',
    body: (
      <>
        <p>
          Depending on where you live (for example under the GDPR, the UK GDPR or the CCPA), you
          can ask to access, correct, delete or export your personal data, and object to or ask us
          to restrict how we use it. You can also complain to your local data protection
          authority.
        </p>
        <p>
          To make a request, open an issue at{' '}
          <ExternalLink href={CONTACT_URL}>our GitHub repository</ExternalLink> saying what you&apos;d
          like done. Don&apos;t include personal details in the public issue; we&apos;ll arrange a
          private way to verify the request. We&apos;ll respond within 30 days.
        </p>
      </>
    ),
  },
  {
    id: 'security',
    heading: 'Security',
    body: (
      <p>
        Passwords are stored only as bcrypt hashes, traffic to the site is encrypted with HTTPS,
        and internal services such as the database and metrics aren&apos;t exposed through the
        public site. No system is perfectly secure, so we can&apos;t guarantee absolute security.
      </p>
    ),
  },
  {
    id: 'children',
    heading: 'Children',
    body: (
      <p>
        The site isn&apos;t aimed at children. Accounts are for people at least 13 years old, or
        the minimum age required in their country. If you believe a child has given us personal
        data, contact us and we&apos;ll delete it.
      </p>
    ),
  },
  {
    id: 'changes',
    heading: 'Changes to this policy',
    body: (
      <p>
        If we change how we handle personal data, we&apos;ll update this page and its effective
        date. Significant changes will also be noted in the{' '}
        <ExternalLink href="https://github.com/fmolinar/arium">repository</ExternalLink>. The{' '}
        <Link to="/terms" className="text-accent hover:underline">
          Terms of Service
        </Link>{' '}
        cover the rules for using the site.
      </p>
    ),
  },
]

export default function Privacy() {
  return (
    <LegalPage
      label="PRIVACY"
      title="Privacy Policy"
      intro={`This policy explains what personal data ${SITE_NAME} collects, why, and what you can do about it.`}
      sections={SECTIONS}
      related={{ to: '/terms', label: 'Terms of Service' }}
    />
  )
}
