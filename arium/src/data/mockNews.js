export const TOPICS = [
  { slug: 'devops', label: 'DevOps' },
  { slug: 'sre', label: 'SRE' },
  { slug: 'gitops', label: 'GitOps' },
  { slug: 'devsecops', label: 'DevSecOps' },
]

// Placeholder headlines until a real aggregation pipeline exists.
export const MOCK_NEWS = [
  {
    id: 'n1',
    title: 'Kubernetes 1.32 ships in-place pod resource resizing as stable',
    summary:
      'Workloads can now have CPU and memory limits adjusted without a restart. The feature graduated after two release cycles in beta.',
    source: 'kubernetes.io',
    url: 'https://kubernetes.io/blog',
    tags: ['devops', 'sre'],
    publishedAt: '2026-09-15T09:00:00Z',
  },
  {
    id: 'n2',
    title: 'Argo CD adds native OCI artifact support for Helm charts',
    summary:
      'GitOps pipelines can now pull chart artifacts straight from OCI registries, cutting out a Helm repo hop for most teams.',
    source: 'argo-cd.readthedocs.io',
    url: 'https://argo-cd.readthedocs.io',
    tags: ['gitops', 'devops'],
    publishedAt: '2026-09-15T07:30:00Z',
  },
  {
    id: 'n3',
    title: 'CISA flags new supply-chain attack pattern in CI runner images',
    summary:
      'Compromised base images used by several self-hosted CI runners were found exfiltrating build secrets. Rotate long-lived tokens now.',
    source: 'cisa.gov',
    url: 'https://www.cisa.gov',
    tags: ['devsecops'],
    publishedAt: '2026-09-14T18:15:00Z',
  },
  {
    id: 'n4',
    title: 'Google SRE team publishes updated error budget policy template',
    summary:
      'The new template accounts for multi-region failover scenarios and gives clearer guidance on burn-rate alerting thresholds.',
    source: 'sre.google',
    url: 'https://sre.google',
    tags: ['sre'],
    publishedAt: '2026-09-14T12:00:00Z',
  },
  {
    id: 'n5',
    title: 'Flux and Argo CD announce shared GitOps interoperability spec',
    summary:
      'The two largest GitOps controllers are converging on a common resource format, easing migration between the two ecosystems.',
    source: 'cncf.io',
    url: 'https://www.cncf.io',
    tags: ['gitops'],
    publishedAt: '2026-09-13T15:45:00Z',
  },
  {
    id: 'n6',
    title: 'OpenTelemetry collector adds native eBPF profiling signal',
    summary:
      'Continuous profiling joins traces, metrics, and logs as a first-class OTel signal, with low-overhead eBPF-based collection.',
    source: 'opentelemetry.io',
    url: 'https://opentelemetry.io',
    tags: ['sre', 'devops'],
    publishedAt: '2026-09-13T10:20:00Z',
  },
  {
    id: 'n7',
    title: 'Sigstore adoption crosses 50% among top package registries',
    summary:
      'More than half of the largest public package registries now support keyless signing and verification via Sigstore by default.',
    source: 'sigstore.dev',
    url: 'https://www.sigstore.dev',
    tags: ['devsecops'],
    publishedAt: '2026-09-12T08:00:00Z',
  },
  {
    id: 'n8',
    title: 'Terraform 1.10 stabilizes ephemeral resources for secret handling',
    summary:
      'Ephemeral values let providers hand back credentials that never get written to state, closing a long-standing secrets-in-state gap.',
    source: 'hashicorp.com',
    url: 'https://www.hashicorp.com',
    tags: ['devops', 'devsecops'],
    publishedAt: '2026-09-11T16:30:00Z',
  },
  {
    id: 'n9',
    title: 'PagerDuty incident retros now auto-link related runbooks',
    summary:
      'The retro tool scans incident timelines and suggests runbook sections that should be updated based on what actually happened.',
    source: 'pagerduty.com',
    url: 'https://www.pagerduty.com',
    tags: ['sre'],
    publishedAt: '2026-09-10T14:00:00Z',
  },
  {
    id: 'n10',
    title: 'GitHub Actions adds first-class OIDC federation for GitOps deploys',
    summary:
      'Workflows can now assume cloud IAM roles without long-lived secrets, closing a common gap in GitOps deployment pipelines.',
    source: 'github.blog',
    url: 'https://github.blog',
    tags: ['gitops', 'devsecops'],
    publishedAt: '2026-09-10T09:30:00Z',
  },
  {
    id: 'n11',
    title: 'New CNCF survey: 68% of teams now run multi-cluster Kubernetes',
    summary:
      'The annual survey shows a sharp rise in multi-cluster adoption, driven mostly by blast-radius isolation and regional failover needs.',
    source: 'cncf.io',
    url: 'https://www.cncf.io',
    tags: ['devops', 'sre'],
    publishedAt: '2026-09-09T11:00:00Z',
  },
  {
    id: 'n12',
    title: 'Falco 1.0 ships with eBPF-only runtime security by default',
    summary:
      'The kernel-module driver is now opt-in only, with eBPF as the default for detecting anomalous container syscalls in production.',
    source: 'falco.org',
    url: 'https://falco.org',
    tags: ['devsecops'],
    publishedAt: '2026-09-09T08:45:00Z',
  },
  {
    id: 'n13',
    title: 'Grafana adds native SLO burn-rate dashboards',
    summary:
      'A new panel type computes multi-window burn rate directly from Prometheus recording rules, no external SLO tool required.',
    source: 'grafana.com',
    url: 'https://grafana.com',
    tags: ['sre', 'devops'],
    publishedAt: '2026-09-08T17:20:00Z',
  },
  {
    id: 'n14',
    title: 'Crossplane 2.0 reworks composition functions for GitOps workflows',
    summary:
      'Composition functions can now be written in any language via a gRPC interface, decoupling platform teams from a single templating engine.',
    source: 'crossplane.io',
    url: 'https://www.crossplane.io',
    tags: ['gitops', 'devops'],
    publishedAt: '2026-09-08T10:00:00Z',
  },
  {
    id: 'n15',
    title: 'NIST releases updated SSDF guidance for CI/CD pipeline hardening',
    summary:
      'The refreshed Secure Software Development Framework adds explicit guidance for build provenance and signed artifact attestation.',
    source: 'nist.gov',
    url: 'https://www.nist.gov',
    tags: ['devsecops'],
    publishedAt: '2026-09-07T13:15:00Z',
  },
  {
    id: 'n16',
    title: 'Honeycomb: why your on-call rotation is probably too long',
    summary:
      'A look at incident response data across hundreds of teams suggests rotations longer than a week correlate with slower MTTR.',
    source: 'honeycomb.io',
    url: 'https://www.honeycomb.io',
    tags: ['sre'],
    publishedAt: '2026-09-07T09:00:00Z',
  },
]
