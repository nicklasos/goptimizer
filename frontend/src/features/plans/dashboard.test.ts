import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { DAY_COLOR, dayKind } from './days'
import { HEALTH, HEALTH_ORDER, summaryCounts, withRunQueued } from './health'
import { chartBars } from './runChart'
import { objectFolder, objectName, sourceSummary } from './source'
import type { Dashboard, DashboardPlan, Health, Plan, Run } from './types'

const HEALTHS: Health[] = ['restoring', 'running', 'failed', 'paused', 'overdue', 'never', 'ok']

describe('health', () => {
  it('has a look for every health, and the summary order lists each once', () => {
    assert.deepEqual(Object.keys(HEALTH).sort(), [...HEALTHS].sort())
    assert.deepEqual([...HEALTH_ORDER].sort(), [...HEALTHS].sort())
  })

  it('colours ok green, failed red, overdue amber, running blue, restoring purple, paused and never grey', () => {
    assert.equal(HEALTH.ok.color, 'colorSuccess')
    assert.equal(HEALTH.failed.color, 'colorError')
    assert.equal(HEALTH.overdue.color, 'colorWarning')
    assert.equal(HEALTH.running.color, 'colorPrimary')
    assert.equal(HEALTH.paused.color, 'colorTextTertiary')
    assert.equal(HEALTH.never.color, 'colorTextTertiary')
    assert.equal(HEALTH.running.spin, true)
    assert.equal(HEALTH.restoring.color, 'purple')
    assert.equal(HEALTH.restoring.tag, 'purple')
  })

  it('summarises only the healths some plan has, problems first', () => {
    assert.deepEqual(summaryCounts({ ok: 5, failed: 1, overdue: 0, restoring: 1, running: 0, paused: 2, never: 0 }), [
      { health: 'failed', count: 1 },
      { health: 'restoring', count: 1 },
      { health: 'ok', count: 5 },
      { health: 'paused', count: 2 },
    ])
    assert.deepEqual(summaryCounts({ ok: 0, failed: 0, overdue: 0, restoring: 0, running: 0, paused: 0, never: 0 }), [])
  })

  it('marks a plan running as soon as its run is queued', () => {
    const plan = (id: number, health: Health) => ({ id, health, last_run: null }) as unknown as DashboardPlan
    const dashboard: Dashboard = { plans: [plan(1, 'ok'), plan(2, 'failed')], summary: { ok: 1, failed: 1, overdue: 0, restoring: 0, running: 0, paused: 0, never: 0 } }
    const run = { id: 9, status: 'queued', trigger: 'manual', started_at: null, finished_at: null, size_bytes: null } as Run

    const next = withRunQueued(dashboard, 2, run)

    assert.equal(next.plans[1].health, 'running')
    assert.equal(next.plans[1].last_run?.id, 9)
    assert.equal(next.plans[0], dashboard.plans[0])
    assert.deepEqual(next.summary, { ok: 1, failed: 0, overdue: 0, restoring: 0, running: 1, paused: 0, never: 0 })
    assert.equal(withRunQueued(dashboard, 3, run), dashboard, 'a plan that is not on the dashboard changes nothing')
  })
})

describe('day strip', () => {
  it('is red for a day with any failure, green for only successes, grey for none', () => {
    assert.equal(dayKind({ date: '2026-10-14', succeeded: 2, failed: 1 }), 'failed')
    assert.equal(dayKind({ date: '2026-10-14', succeeded: 2, failed: 0 }), 'succeeded')
    assert.equal(dayKind({ date: '2026-10-14', succeeded: 0, failed: 0 }), 'none')
    assert.equal(DAY_COLOR.failed, 'colorError')
    assert.equal(DAY_COLOR.succeeded, 'colorSuccess')
  })
})

describe('runs chart', () => {
  const run = (id: number, size: number | null, status: Run['status'] = 'succeeded') => ({ id, size_bytes: size, status }) as Run
  const box = { width: 300, height: 100, slots: 30, gap: 2, minHeight: 4 }

  it('draws the newest run on the right and scales heights to the largest archive', () => {
    const bars = chartBars([run(3, 50), run(2, null, 'failed'), run(1, 100)], box)
    assert.deepEqual(
      bars.map((bar) => [bar.run.id, bar.x, bar.y, bar.width, bar.height]),
      [
        [1, 270, 0, 8, 100],
        [2, 280, 96, 8, 4],
        [3, 290, 50, 8, 50],
      ]
    )
  })

  it('gives a tiny archive the least height, and draws only as many runs as slots', () => {
    const bars = chartBars([run(2, 1), run(1, 1_000_000)], box)
    assert.equal(bars[1].height, 4)
    const many = Array.from({ length: 40 }, (_, i) => run(40 - i, 10))
    const drawn = chartBars(many, box)
    assert.equal(drawn.length, 30)
    assert.equal(drawn[0].run.id, 11)
    assert.equal(drawn[29].run.id, 40)
    assert.equal(drawn[0].x, 0)
  })

  it('draws nothing without runs, and every bar at least a pixel wide', () => {
    assert.deepEqual(chartBars([], box), [])
    assert.equal(chartBars([run(1, 5)], { ...box, width: 30, gap: 4 })[0].width, 1)
  })
})

describe('source summary', () => {
  it('names the database or the paths', () => {
    assert.equal(sourceSummary({ postgres: { host: 'db', port: 5432, user: 'shop', database: 'shop', ssl_mode: 'disable' } } as Plan), 'shop@db:5432/shop')
    assert.equal(sourceSummary({ sqlite: { path: '/srv/app.db' } } as Plan), '/srv/app.db')
    assert.equal(sourceSummary({ files: { paths: ['/srv/a', '/srv/b'], excludes: [] } } as unknown as Plan), '/srv/a, /srv/b')
  })

  it('splits an object key into its folder and name', () => {
    assert.equal(objectFolder('shop/shop-1.db.gz'), 'shop/')
    assert.equal(objectName('shop/shop-1.db.gz'), 'shop-1.db.gz')
    assert.equal(objectFolder('top.gz'), '')
  })
})
