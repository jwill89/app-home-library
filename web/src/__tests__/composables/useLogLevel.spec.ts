import { describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { LogLevel } from '@/types'
import { logLevelLabel } from '@/app/format'
import { logLevels } from '@/constants'
import { useLogLevel } from '@/composables/useLogLevel'
import { ProblemError } from '@/app/problem'

// The server's answers as the API client hands them over.
const answered = (level: LogLevel, started: LogLevel) =>
  ({ data: { level, started }, error: undefined, response: new Response() }) as never
const refused = (status: number, title: string) =>
  ({
    data: undefined,
    error: { type: 'about:blank', status, title },
    response: new Response(null, { status }),
  }) as never

describe('useLogLevel', () => {
  it("loads the server's level and the one it started with", async () => {
    const get = vi.spyOn(api, 'GET').mockResolvedValue(answered('warn', 'info'))
    const log = useLogLevel()
    expect(log.level.value).toBeNull()
    await log.load()
    expect(get).toHaveBeenCalledWith('/log-level')
    expect(log.level.value).toBe('warn')
    expect(log.started.value).toBe('info')
  })

  it('throws what the server said when it will not tell', async () => {
    vi.spyOn(api, 'GET').mockResolvedValue(refused(403, 'Forbidden'))
    const log = useLogLevel()
    const failed = log.load()
    await expect(failed).rejects.toBeInstanceOf(ProblemError)
    await expect(failed).rejects.toMatchObject({ status: 403, message: 'Forbidden' })
    expect(log.level.value).toBeNull()
  })

  it('sets a new level, shown at once and then as the server has it', async () => {
    vi.spyOn(api, 'GET').mockResolvedValue(answered('info', 'info'))
    const put = vi.spyOn(api, 'PUT').mockResolvedValue(answered('debug', 'info'))
    const log = useLogLevel()
    await log.load()
    const saving = log.choose('debug')
    expect(log.level.value).toBe('debug')
    await saving
    expect(put).toHaveBeenCalledWith('/log-level', { body: { level: 'debug' } })
    expect(log.level.value).toBe('debug')
    expect(log.started.value).toBe('info')
  })

  it('goes back to the level it had when the save fails, and says why', async () => {
    vi.spyOn(api, 'GET').mockResolvedValue(answered('info', 'warn'))
    vi.spyOn(api, 'PUT').mockResolvedValue(refused(503, 'No connection'))
    const log = useLogLevel()
    await log.load()
    await expect(log.choose('error')).rejects.toMatchObject({
      name: 'ProblemError',
      status: 503,
      message: 'No connection',
    })
    expect(log.level.value).toBe('info')
    expect(log.started.value).toBe('warn')
  })
})

describe('logLevels', () => {
  it('runs from the most the log holds to the least, each named as the choice shows it', () => {
    expect(logLevels.map((l) => l.value)).toEqual(['debug', 'info', 'warn', 'error'])
    expect(logLevelLabel('debug')).toBe('Everything (debug)')
    expect(logLevelLabel('info')).toBe('What it does (info)')
    expect(logLevelLabel('warn')).toBe('Problems (warn)')
    expect(logLevelLabel('error')).toBe('Failures only (error)')
  })
})
