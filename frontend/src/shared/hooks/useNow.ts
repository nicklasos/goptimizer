import { useEffect, useState } from 'react'

/** The current time, renewed every `intervalMs`, so "3 hours ago" stays true on a page left open. */
export function useNow(intervalMs = 60_000): Date {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), intervalMs)
    return () => window.clearInterval(timer)
  }, [intervalMs])
  return now
}
