import { Link } from 'react-router-dom'
import LegalPage, { ExternalLink, List } from '../components/LegalPage'
import { CONTACT_URL, OPERATOR, SITE_NAME, SITE_URL } from '../legal/config'

const SECTIONS = [
  {
    id: 'acceptance',
    heading: 'Accepting these terms',
    body: (
      <p>
        By using {SITE_NAME} ({SITE_URL}, the &quot;site&quot;), you agree to these Terms of
        Service. If you don&apos;t agree, please don&apos;t use the site. The site is run by{' '}
        {OPERATOR} (&quot;we&quot;, &quot;us&quot;).
      </p>
    ),
  },
  {
    id: 'service',
    heading: 'What the site is',
    body: (
      <>
        <p>
          {SITE_NAME} is a free, non-commercial personal project that collects publicly published
          DevOps, SRE, GitOps and DevSecOps news into one feed. It&apos;s also a portfolio piece,
          so features may change, break or disappear without notice.
        </p>
        <p>
          The site is provided for general information only. Nothing on it is professional,
          security or legal advice.
        </p>
      </>
    ),
  },
  {
    id: 'third-party-content',
    heading: 'News from other publishers',
    body: (
      <>
        <p>
          The News Hub shows headlines, short summaries and links collected automatically from
          public RSS/Atom feeds and the Hacker News search API. That content belongs to its
          original publishers and authors. We link to the original article and name its source,
          and we don&apos;t claim ownership of it or endorse it.
        </p>
        <p>
          Collection is automated, so we can&apos;t guarantee that items are accurate, complete or
          current. Linked sites have their own terms and privacy practices, which we don&apos;t
          control.
        </p>
        <p>
          If you publish content that appears here and want it removed, or you believe something
          on the site infringes your rights, open an issue at{' '}
          <ExternalLink href={CONTACT_URL}>our GitHub repository</ExternalLink> with a link to the
          item, and we&apos;ll review it promptly.
        </p>
      </>
    ),
  },
  {
    id: 'accounts',
    heading: 'Accounts',
    body: (
      <>
        <p>
          Some features may require an account. If you create one, you must give accurate
          information, keep your password secret, and are responsible for activity under your
          account. You must be at least 13 years old, or the minimum age required in your country
          to consent to online services, to create an account.
        </p>
        <p>
          We may suspend or delete accounts that break these terms. You can ask us to delete your
          account at any time, as described in the{' '}
          <Link to="/privacy" className="text-accent hover:underline">
            Privacy Policy
          </Link>
          .
        </p>
      </>
    ),
  },
  {
    id: 'acceptable-use',
    heading: 'Acceptable use',
    body: (
      <>
        <p>When using the site, you agree not to:</p>
        <List
          items={[
            'break the law, or infringe anyone else’s rights;',
            'attempt to gain unauthorized access to the site, its API or its infrastructure, or test their security without written permission;',
            'disrupt the service, including by sending excessive automated requests or scraping at a rate that degrades it for others;',
            'upload malware, or use the site to send spam, harass people, or impersonate anyone;',
            'misrepresent third-party content shown here as your own.',
          ]}
        />
        <p>
          Found a security issue? Please don&apos;t post the details publicly. Open an issue
          asking for a private contact, and we&apos;ll follow up.
        </p>
      </>
    ),
  },
  {
    id: 'user-content',
    heading: 'Content you post',
    body: (
      <p>
        If the site lets you post content (for example in the planned forum), you keep ownership
        of it. You give us a non-exclusive, royalty-free licence to host, display and distribute it
        on the site for as long as it stays there. You confirm that you have the right to post it.
        We may remove content that breaks these terms.
      </p>
    ),
  },
  {
    id: 'our-content',
    heading: 'The site’s own code and design',
    body: (
      <p>
        The site&apos;s source code is published on{' '}
        <ExternalLink href="https://github.com/fmolinar/arium">GitHub</ExternalLink>. Your rights
        to reuse it are set by the licence in that repository, if there is one. Otherwise, all
        rights are reserved. The {SITE_NAME} name and design may not be used to suggest that we
        endorse you or your project.
      </p>
    ),
  },
  {
    id: 'disclaimer',
    heading: 'No warranty',
    body: (
      <p>
        The site is provided &quot;as is&quot; and &quot;as available&quot;, without warranties of
        any kind, express or implied, including fitness for a particular purpose, accuracy and
        uninterrupted availability, to the fullest extent the law allows.
      </p>
    ),
  },
  {
    id: 'liability',
    heading: 'Limitation of liability',
    body: (
      <p>
        To the fullest extent the law allows, we aren&apos;t liable for any indirect, incidental,
        special or consequential damages, or for loss of data, profits or goodwill, arising from
        your use of the site or of linked third-party content. The site is free, and our total
        liability for any claim relating to it is limited to zero. Nothing in these terms limits
        liability that can&apos;t be limited by law.
      </p>
    ),
  },
  {
    id: 'changes',
    heading: 'Changes and termination',
    body: (
      <p>
        We may update these terms. When we do, we&apos;ll change the effective date at the top of
        this page, and continued use of the site after that means you accept the new terms. We may
        change, suspend or shut down the site at any time.
      </p>
    ),
  },
  {
    id: 'contact',
    heading: 'Contact',
    body: (
      <p>
        Questions, takedown requests and other notices: open an issue at{' '}
        <ExternalLink href={CONTACT_URL}>{CONTACT_URL.replace('https://', '')}</ExternalLink>.
      </p>
    ),
  },
]

export default function Terms() {
  return (
    <LegalPage
      label="TERMS"
      title="Terms of Service"
      intro={`These terms explain the rules for using ${SITE_NAME}. They're written to be readable; please read them in full.`}
      sections={SECTIONS}
      related={{ to: '/privacy', label: 'Privacy Policy' }}
    />
  )
}
