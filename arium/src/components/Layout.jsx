import { NavLink, Outlet } from 'react-router-dom'
import ThemeToggle from './ThemeToggle'

const NAV_LINKS = [
  { to: '/news', label: 'NEWS' },
  { to: '/forum', label: 'FORUM' },
  { to: '/about', label: 'ABOUT' },
]

function navLinkClass({ isActive }) {
  return [
    'font-mono text-sm tracking-wide transition-colors',
    isActive ? 'text-accent' : 'text-text hover:text-text-h',
  ].join(' ')
}

export default function Layout() {
  return (
    <div className="flex min-h-screen flex-col bg-bg text-text">
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <NavLink to="/" className="font-mono text-lg font-bold text-text-h">
            arium<span className="text-accent">_</span>
          </NavLink>

          <nav className="flex items-center gap-6">
            {NAV_LINKS.map(({ to, label }) => (
              <NavLink key={to} to={to} className={navLinkClass}>
                [ {label} ]
              </NavLink>
            ))}
            <ThemeToggle />
          </nav>
        </div>
      </header>

      <main className="flex-1">
        <Outlet />
      </main>

      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-6xl flex-col gap-1 px-6 py-6 font-mono text-xs text-text sm:flex-row sm:items-center sm:justify-between">
          <span>© {new Date().getFullYear()} arium</span>
          <span>DevOps · SRE · GitOps · DevSecOps</span>
        </div>
      </footer>
    </div>
  )
}
