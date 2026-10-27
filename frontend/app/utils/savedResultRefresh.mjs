// Coalesce save notifications without postponing refresh indefinitely. If more
// saves arrive during a fetch, fetch once more after it completes.
export function createSavedResultRefresh(refresh, delay = 150) {
  let timer, running = false, dirty = false, disposed = false
  function request() {
    if (disposed) return
    dirty = true
    if (!running && timer === undefined) timer = setTimeout(flush, delay)
  }
  async function flush() {
    timer = undefined
    if (disposed || !dirty) return
    dirty = false
    running = true
    try { await refresh() }
    finally {
      running = false
      if (dirty && !disposed) request()
    }
  }
  function cancel() { clearTimeout(timer); timer = undefined; dirty = false }
  return { request, cancel, dispose() { disposed = true; cancel() } }
}
