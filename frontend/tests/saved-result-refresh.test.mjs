import { test } from 'node:test'
import assert from 'node:assert/strict'
import { setTimeout as sleep } from 'node:timers/promises'
import { createSavedResultRefresh } from '../app/utils/savedResultRefresh.mjs'

test('refreshes saved results before a run ends, coalesces bursts and catches saves during a fetch', async () => {
  let calls = 0, release, started
  const firstStarted = new Promise(resolve => { started = resolve })
  const gate = new Promise(resolve => { release = resolve })
  const refresh = createSavedResultRefresh(async () => { calls++; if (calls === 1) { started(); await gate } }, 1)
  for (let i = 0; i < 12; i++) refresh.request()
  await firstStarted
  assert.equal(calls, 1)
  for (let i = 0; i < 12; i++) refresh.request()
  await sleep(10)
  assert.equal(calls, 1, 'must not fetch concurrently')
  release()
  await sleep(20)
  assert.equal(calls, 2, 'must load saves that arrived during the first fetch')
  refresh.request()
  refresh.dispose()
  await sleep(10)
  assert.equal(calls, 2, 'unmounted page must not keep refreshing')
})

test('end-of-run cancels scheduled work but later runs can refresh', async () => {
  let calls = 0
  const refresh = createSavedResultRefresh(async () => { calls++ }, 1)
  refresh.request(); refresh.cancel()
  await sleep(10); assert.equal(calls, 0)
  refresh.request()
  await sleep(10); assert.equal(calls, 1)
  refresh.dispose()
})
