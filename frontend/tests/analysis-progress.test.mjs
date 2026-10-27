import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import assert from 'node:assert/strict'

// Exercise the actual progress callback without requiring a running Wails app.
const source = readFileSync(new URL('../app/pages/search.vue', import.meta.url), 'utf8')
const callback = source.match(/EventsOn\('analyze:progress', \(p: AnalysisProgress\) => \{([\s\S]*?)\n  \}\)/)?.[1]
assert.ok(callback, 'analysis progress callback must exist')

test('a successful image clears the current error and preserves failure history', () => {
  const state = {
    analyzing: { value: true }, clockNow: { value: 0 }, progress: { value: null },
    runSucceeded: { value: 0 }, runFailed: { value: 0 },
    status: { value: { analyzed: 10 } }, analyzedBeforeRun: { value: 10 },
    failedItems: { value: [] }, error: { value: '' }, stoppedEarly: { value: false },
    stopMessage: { value: '' },
    scheduleSavedRefresh: () => { refreshes++ },
  }
  let refreshes = 0
  const update = new Function(...Object.keys(state), 'p', callback).bind(null, ...Object.values(state))
  update({ done: 1, total: 3, name: 'a.jpg', status: 'failed', succeeded: 0, failed: 1, message: 'quota', stopped: false })
  assert.equal(state.error.value, 'a.jpg: quota')
  assert.equal(refreshes, 0)
  update({ done: 2, total: 3, name: 'b.jpg', status: 'moved', succeeded: 1, failed: 1, message: '', stopped: false })
  assert.equal(state.error.value, '')
  assert.equal(state.failedItems.value.length, 1)
  assert.equal(state.status.value.analyzed, 11)
  assert.equal(state.runSucceeded.value, 1)
  assert.equal(state.runFailed.value, 1)
  assert.equal(refreshes, 1)
  update({ done: 3, total: 5, name: 'fatal.jpg', status: 'failed', succeeded: 1, failed: 2, message: 'quota exhausted', stopped: true })
  update({ done: 4, total: 5, name: 'completed.jpg', status: 'moved', succeeded: 2, failed: 2, message: '', stopped: false })
  assert.equal(state.error.value, 'fatal.jpg: quota exhausted')
  assert.equal(state.stopMessage.value, 'fatal.jpg: quota exhausted')
  assert.equal(state.status.value.analyzed, 12)
  state.analyzing.value = false
  update({ done: 3, total: 3, name: 'c.jpg', status: 'failed', succeeded: 1, failed: 2, message: 'late event', stopped: false })
  assert.equal(state.error.value, 'fatal.jpg: quota exhausted')
  assert.equal(state.failedItems.value.length, 2)
  assert.equal(refreshes, 2)
})
