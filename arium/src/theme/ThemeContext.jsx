import { useCallback, useEffect, useState } from 'react'
import { ThemeContext } from './context'

const STORAGE_KEY = 'arium-theme'

function getStoredTheme() {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored === 'light' || stored === 'dark' ? stored : null
  } catch {
    return null
  }
}

export function ThemeProvider({ children }) {
  const [theme, setTheme] = useState(() => getStoredTheme())

  useEffect(() => {
    const root = document.documentElement
    if (theme) {
      root.setAttribute('data-theme', theme)
    } else {
      root.removeAttribute('data-theme')
    }

    try {
      if (theme) {
        localStorage.setItem(STORAGE_KEY, theme)
      } else {
        localStorage.removeItem(STORAGE_KEY)
      }
    } catch {
      // localStorage unavailable (private browsing, blocked storage) - theme still applies for this load
    }
  }, [theme])

  const toggleTheme = useCallback(() => {
    setTheme((current) => {
      if (current === 'dark') return 'light'
      if (current === 'light') return 'dark'
      // No explicit choice yet: flip relative to the current system preference.
      const systemIsDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      return systemIsDark ? 'light' : 'dark'
    })
  }, [])

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>{children}</ThemeContext.Provider>
  )
}
