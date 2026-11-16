import { ref } from 'vue'
import { api } from '@/api/client'
import type { LogLevel } from '@/types'
import { ProblemError } from '@/app/problem'

// How much the server writes to its log, which the owner turns up to find out what went wrong
// and down again after. It holds until the server restarts, which goes back to the level the
// server was started with. The levels to choose from are constants/index.ts's logLevels.

/** The server's log level, and the one a restart goes back to; null until loaded. */
export function useLogLevel() {
  const level = ref<LogLevel | null>(null)
  const started = ref<LogLevel | null>(null)

  function take(answer: { level: LogLevel; started: LogLevel }) {
    level.value = answer.level
    started.value = answer.started
  }

  async function load() {
    const { data, error } = await api.GET('/log-level')
    if (error) {
      throw new ProblemError(error)
    }
    take(data)
  }

  /** Sets the server's level at once; when it is refused, the level shown is the server's again. */
  async function choose(next: LogLevel) {
    const before = level.value
    level.value = next
    const { data, error } = await api.PUT('/log-level', { body: { level: next } })
    if (error) {
      level.value = before
      throw new ProblemError(error)
    }
    take(data)
  }

  return { level, started, load, choose }
}
