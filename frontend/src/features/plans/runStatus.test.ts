import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { anyRunActive, dashboardPollMs, DASHBOARD_POLL_MS, isPlanBusy, isRunActive } from './runStatus'
import type { Dashboard, DashboardPlan, Health, RunStatus } from './types'

const run = (status: RunStatus) => ({ status })

describe('run status', () => {
  it('is active while queued or running', () => {
    assert.equal(isRunActive(run('queued')), true)
    assert.equal(isRunActive(run('running')), true)
    assert.equal(isRunActive(run('succeeded')), false)
    assert.equal(isRunActive(run('failed')), false)
    assert.equal(isRunActive(null), false)
    assert.equal(isRunActive(undefined), false)
  })

  it('polls a page only while one of its runs is in progress', () => {
    assert.equal(anyRunActive(undefined), false)
    assert.equal(anyRunActive([]), false)
    assert.equal(anyRunActive([run('failed'), run('succeeded')]), false)
    assert.equal(anyRunActive([run('succeeded'), run('running')]), true)
  })

  it('polls the dashboard fast while a plan runs', () => {
    const dashboard = (...healths: Health[]) => ({ plans: healths.map((health) => ({ health }) as DashboardPlan) }) as Dashboard
    assert.equal(dashboardPollMs(undefined), DASHBOARD_POLL_MS.idle)
    assert.equal(dashboardPollMs(dashboard('ok', 'failed', 'overdue')), DASHBOARD_POLL_MS.idle)
    assert.equal(dashboardPollMs(dashboard('ok', 'running')), DASHBOARD_POLL_MS.active)
    assert.equal(dashboardPollMs(dashboard('ok', 'restoring')), DASHBOARD_POLL_MS.active, 'and while one is restored')
  })

  it('counts a plan busy while it runs or is restored', () => {
    assert.equal(isPlanBusy(undefined), false)
    assert.equal(isPlanBusy({ health: 'ok', last_run: run('succeeded') }), false)
    assert.equal(isPlanBusy({ health: 'running', last_run: run('queued') }), true)
    assert.equal(isPlanBusy({ health: 'restoring', last_run: run('succeeded') }), true)
  })
})
