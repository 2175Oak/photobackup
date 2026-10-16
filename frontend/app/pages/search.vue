<template>
  <section>
    <header class="pagehead">
      <h1>ค้นหารูป</h1>
      <p class="lede">ให้ AI (Gemini) อธิบายรูปและตั้งแท็กให้ แล้วค้นหารูปจากคำอธิบายหรือแท็กได้</p>
    </header>

    <p v-if="error" class="notice notice-danger" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice notice-ok" role="status">{{ notice }}</p>

    <p v-if="!dest" class="empty-state">
      ยังไม่ได้เลือกโฟลเดอร์ปลายทาง <NuxtLink to="/">ไปเลือกที่หน้าย้ายรูป</NuxtLink>
    </p>

    <template v-else>
      <!-- วิเคราะห์ภาพ -->
      <div class="panel analyze">
        <div class="analyze-row">
          <div>
            <h2>วิเคราะห์ภาพด้วย AI</h2>
            <p v-if="status">
              วิเคราะห์แล้ว <strong>{{ status.analyzed }}</strong> จาก {{ status.total }} รูป
              <template v-if="pendingCount > 0">· เหลือ {{ pendingCount }} รูป</template>
            </p>
          </div>
          <div class="analyze-actions">
            <button
              v-if="!analyzing"
              type="button"
              class="btn btn-primary"
              :disabled="busy || !status?.configured || pendingCount === 0"
              @click="analyze"
            >
              วิเคราะห์รูปที่ยังไม่มีคำอธิบาย
            </button>
            <button v-else type="button" class="btn btn-quiet" @click="CancelAnalyze()">หยุด</button>
          </div>
        </div>

        <p v-if="status && !status.configured" class="notice notice-warn" role="alert">
          ยังไม่ได้ตั้งค่า <code>GEMINI_API_KEY</code> ในไฟล์ <code>.env</code> (วางไว้โฟลเดอร์เดียวกับโปรแกรม) แล้วเปิดโปรแกรมใหม่
        </p>
        <p v-else-if="status && status.total === 0" class="hint-inline">ยังไม่มีรูปในประวัติ ย้ายรูปมาปลายทางก่อนจึงจะวิเคราะห์ได้</p>

        <div v-if="analyzing" class="progress-wrap">
          <progress v-if="progress" :value="progress.done" :max="progress.total" aria-label="ความคืบหน้าการวิเคราะห์" />
          <progress v-else aria-label="กำลังวิเคราะห์" />
          <span class="progress-name">
            <template v-if="progress">กำลังวิเคราะห์ {{ progress.done }} จาก {{ progress.total }} รูป: {{ progress.name }}</template>
            <template v-else>กำลังเริ่มวิเคราะห์…</template>
          </span>
        </div>

        <details v-if="failedItems.length" class="failed">
          <summary>วิเคราะห์ไม่สำเร็จ {{ failedItems.length }} รูป (กดวิเคราะห์ซ้ำได้)</summary>
          <ul>
            <li v-for="f in failedItems" :key="f.name"><strong>{{ f.name }}</strong> {{ f.message }}</li>
          </ul>
        </details>
      </div>

      <!-- ค้นหา -->
      <div class="modes" role="tablist" aria-label="วิธีค้นหา">
        <button type="button" role="tab" :aria-selected="mode === 'caption'" :class="{ on: mode === 'caption' }" @click="mode = 'caption'">
          ค้นหาจากคำอธิบาย
        </button>
        <button type="button" role="tab" :aria-selected="mode === 'tag'" :class="{ on: mode === 'tag' }" @click="mode = 'tag'">
          ค้นหาจากแท็ก
        </button>
      </div>

      <form v-if="mode === 'caption'" class="searchbox" role="search" @submit.prevent="runSearch">
        <label class="sr-only" for="q">คำค้นหา</label>
        <input id="q" v-model="text" type="search" placeholder="พิมพ์คำที่อยู่ในคำอธิบายภาพ เช่น ทะเล, เด็กกำลังเล่น" autocomplete="off" />
        <button type="submit" class="btn btn-primary">ค้นหา</button>
        <button v-if="text" type="button" class="btn btn-text" @click="text = ''">ล้าง</button>
      </form>
      <p v-if="mode === 'caption'" class="hint-inline">พิมพ์หลายคำได้ คั่นด้วยช่องว่าง รูปต้องมีครบทุกคำ</p>

      <div v-else class="cloud-wrap">
        <p v-if="!tags.length" class="empty-state">ยังไม่มีแท็ก ให้กดวิเคราะห์รูปด้านบนก่อน</p>
        <template v-else>
          <ul class="cloud" aria-label="Tag Cloud">
            <li v-for="t in tags" :key="t.name">
              <button
                type="button"
                class="tag-btn"
                :class="{ on: picked.includes(t.name) }"
                :style="{ fontSize: tagSize(t.count) }"
                :aria-pressed="picked.includes(t.name)"
                :title="`${t.count} รูป`"
                @click="toggleTag(t.name)"
              >
                {{ t.name }}<sup>{{ t.count }}</sup>
              </button>
            </li>
          </ul>
          <p class="hint-inline">
            <template v-if="picked.length">
              แท็กที่เลือก: <strong>{{ picked.join(', ') }}</strong> (รูปต้องมีครบทุกแท็ก)
              <button type="button" class="btn btn-text" @click="picked = []">ล้างที่เลือก</button>
            </template>
            <template v-else>กดที่แท็กเพื่อค้นหา เลือกได้หลายแท็ก ตัวใหญ่ = มีรูปมาก</template>
          </p>
        </template>
      </div>

      <!-- ผลลัพธ์ -->
      <p class="count" aria-live="polite">
        <template v-if="searching">กำลังค้นหา…</template>
        <template v-else-if="!showResults">เลือกแท็กเพื่อแสดงรูป</template>
        <template v-else>พบ {{ results.length }} รูป</template>
      </p>

      <ul v-if="showResults && results.length" class="results">
        <li v-for="r in shown" :key="r.id" class="card">
          <span class="mat"><PhotoThumb kind="dest" :name="r.fileName" /></span>
          <div class="body">
            <p class="fname" :title="r.backupPath">{{ r.fileName }}</p>
            <p class="caption">
              <template v-for="(seg, i) in highlight(r.caption)" :key="i">
                <mark v-if="seg.hit">{{ seg.text }}</mark>
                <template v-else>{{ seg.text }}</template>
              </template>
            </p>
            <ul v-if="r.tags?.length" class="chips">
              <li v-for="t in r.tags" :key="t.id">
                <button type="button" :class="{ on: picked.includes(t.name) }" @click="jumpToTag(t.name)">{{ t.name }}</button>
              </li>
            </ul>
          </div>
        </li>
      </ul>
      <p v-else-if="showResults && !searching" class="empty-state">
        {{ status && status.analyzed === 0 ? 'ยังไม่มีรูปที่วิเคราะห์ ให้กดวิเคราะห์ด้านบนก่อน' : 'ไม่พบรูปที่ตรงกับคำค้นหา' }}
      </p>

      <p v-if="results.length > limit" class="more">
        <button type="button" class="btn btn-quiet" @click="limit += 40">แสดงเพิ่ม (อีก {{ results.length - limit }} รูป)</button>
      </p>
    </template>
  </section>
</template>

<script setup lang="ts">
import {
  AnalyzePending,
  CancelAnalyze,
  GetAnalysisStatus,
  ListTags,
  SearchPhotos,
} from '~~/wailsjs/go/service/PhotoService'
import { EventsOn } from '~~/wailsjs/runtime/runtime'
import type { model, service } from '~~/wailsjs/go/models'

type TagCount = { name: string; count: number }

useHead({ title: 'ค้นหารูป | Photo Backup' })

const { dest } = useFolders()

const mode = ref<'caption' | 'tag'>('caption')
const text = ref('')
const picked = ref<string[]>([])
const tags = ref<TagCount[]>([])
const results = ref<model.Photo[]>([])
const status = ref<service.AnalysisStatus | null>(null)
const failedItems = ref<service.FileResult[]>([])
const progress = ref<{ done: number; total: number; name: string } | null>(null)

const limit = ref(40)
const searching = ref(false)
const analyzing = ref(false)
const error = ref('')
const notice = ref('')

const busy = computed(() => analyzing.value)
const pendingCount = computed(() => (status.value ? status.value.total - status.value.analyzed : 0))
const shown = computed(() => results.value.slice(0, limit.value))
// โหมดแท็กต้องเลือกแท็กก่อนจึงแสดงรูป (ไม่งั้นจะแสดงรูปทั้งหมดซ้ำกับโหมดคำอธิบาย)
const showResults = computed(() => mode.value === 'caption' || picked.value.length > 0)

async function loadStatus() {
  try {
    status.value = await GetAnalysisStatus()
  } catch (e) {
    error.value = errText(e)
  }
}

async function loadTags() {
  try {
    tags.value = ((await ListTags()) as TagCount[] ?? []).slice().sort((a: TagCount, b: TagCount) => a.name.localeCompare(b.name, 'th'))
    // แท็กที่เลือกไว้แต่ไม่มีแล้ว (เช่น ล้างประวัติ) ให้เอาออก
    picked.value = picked.value.filter((p) => tags.value.some((t: TagCount) => t.name === p))
  } catch (e) {
    error.value = errText(e)
  }
}

let searchSeq = 0
async function runSearch() {
  const seq = ++searchSeq // ผลของการค้นที่ช้ากว่าและเก่ากว่าต้องไม่มาทับผลล่าสุด
  limit.value = 40
  if (!dest.value || !showResults.value) {
    results.value = []
    return
  }
  searching.value = true
  try {
    const r = await SearchPhotos(mode.value === 'caption' ? text.value : '', mode.value === 'tag' ? picked.value : [])
    if (seq === searchSeq) {
      results.value = r ?? []
      error.value = ''
    }
  } catch (e) {
    if (seq === searchSeq) {
      results.value = []
      error.value = errText(e)
    }
  } finally {
    if (seq === searchSeq) searching.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadStatus(), loadTags()])
  await runSearch()
}

async function analyze() {
  error.value = ''
  notice.value = ''
  failedItems.value = []
  progress.value = null
  analyzing.value = true
  try {
    const r = await AnalyzePending()
    failedItems.value = (r.items ?? []).filter((i: { status: string }) => i.status === 'failed')
    if (r.done === 0 && r.failed > 0) {
      // ไม่สำเร็จเลย แสดงสาเหตุให้เห็นทันที ไม่ต้องกดเปิดรายการ
      error.value = `วิเคราะห์ไม่สำเร็จ: ${failedItems.value[0]?.message ?? 'ไม่ทราบสาเหตุ'}`
    } else notice.value = r.cancelled
      ? `หยุดการวิเคราะห์แล้ว สำเร็จ ${r.done} รูป`
      : `วิเคราะห์เสร็จ สำเร็จ ${r.done} รูป${r.failed ? ` ไม่สำเร็จ ${r.failed} รูป` : ''}`
  } catch (e) {
    error.value = errText(e)
  } finally {
    analyzing.value = false
    progress.value = null
    await refreshAll()
  }
}

function toggleTag(name: string) {
  picked.value = picked.value.includes(name) ? picked.value.filter((n: string) => n !== name) : [...picked.value, name]
}

/** กดแท็กใต้รูปแล้วไปค้นด้วยแท็กนั้นทันที */
function jumpToTag(name: string) {
  mode.value = 'tag'
  picked.value = [name]
}

/** ขนาดตัวอักษรของแท็กใน Tag Cloud ตามจำนวนรูป (0.95em – 2.1em) */
function tagSize(count: number): string {
  const counts = tags.value.map((t: TagCount) => t.count)
  const min = Math.min(...counts)
  const max = Math.max(...counts)
  const ratio = max === min ? 0.4 : (count - min) / (max - min)
  return `${(0.95 + ratio * 1.15).toFixed(2)}em`
}

/** แบ่งคำอธิบายเป็นช่วงๆ เพื่อไฮไลต์คำที่ค้นหา (ไม่ใช้ v-html จึงไม่มีปัญหา HTML แทรก) */
function highlight(caption: string): { text: string; hit: boolean }[] {
  const words: string[] = mode.value === 'caption' ? text.value.trim().split(/\s+/).filter(Boolean) : []
  if (!words.length) return [{ text: caption, hit: false }]
  const re = new RegExp(`(${words.map((w: string) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi')
  return caption
    .split(re)
    .filter(Boolean)
    .map((part) => ({ text: part, hit: words.some((w: string) => w.toLowerCase() === part.toLowerCase()) }))
}

// พิมพ์แล้วค้นให้เองหลังหยุดพิมพ์ครู่หนึ่ง
let timer: ReturnType<typeof setTimeout> | undefined
watch(text, () => {
  clearTimeout(timer)
  timer = setTimeout(runSearch, 300)
})
watch([mode, picked, dest], () => {
  clearTimeout(timer)
  runSearch()
}, { deep: true })
watch(dest, () => {
  failedItems.value = []
  refreshAll()
})

let stopProgress: (() => void) | undefined
onMounted(() => {
  if (!inWails()) return
  stopProgress = EventsOn('analyze:progress', (p: { done: number; total: number; name: string }) => {
    progress.value = p
  })
  refreshAll()
})
onBeforeUnmount(() => {
  stopProgress?.()
  clearTimeout(timer)
})
</script>

<style scoped>
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

.hint-inline {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--ink-soft);
}

.analyze-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px 16px;
}

.analyze-row p {
  margin: 4px 0 0;
}

.progress-wrap {
  display: grid;
  gap: 6px;
  margin-top: 12px;
}

.progress-name {
  font-size: 14px;
  color: var(--ink-soft);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.failed {
  margin-top: 12px;
  font-size: 14px;
}

.failed summary {
  cursor: pointer;
  color: var(--danger);
  font-weight: 600;
}

.failed ul {
  margin: 8px 0 0;
  padding-left: 20px;
  max-height: 160px;
  overflow: auto;
}

.modes {
  display: flex;
  gap: 4px;
  margin: 26px 0 14px;
  border-bottom: 2px solid var(--line);
}

.modes button {
  margin-bottom: -2px;
  padding: 8px 18px;
  border: 0;
  border-bottom: 3px solid transparent;
  background: none;
  color: var(--ink-soft);
  font: 600 16px/1.4 var(--font-body);
  cursor: pointer;
}

.modes button:hover {
  color: var(--ink);
}

.modes button.on {
  border-bottom-color: var(--cyan-600);
  color: var(--cyan-800);
}

.searchbox {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.searchbox input {
  flex: 1 1 280px;
  min-width: 0;
  padding: 9px 14px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #fff;
  color: var(--ink);
  font: 400 16px/1.5 var(--font-body);
}

.searchbox input:focus-visible {
  outline: 3px solid var(--cyan-600);
  outline-offset: 1px;
}

.cloud {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: center;
  gap: 6px 10px;
  margin: 0;
  padding: 18px 16px;
  list-style: none;
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.tag-btn {
  padding: 2px 10px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: none;
  color: var(--cyan-600);
  font-family: var(--font-body);
  font-weight: 600;
  line-height: 1.5;
  cursor: pointer;
}

.tag-btn sup {
  margin-left: 2px;
  font-size: 0.55em;
  font-weight: 400;
  color: var(--ink-soft);
}

.tag-btn:hover {
  background: var(--cyan-100);
}

.tag-btn.on {
  background: var(--pencil);
  border-color: var(--pencil);
  color: var(--ink);
}

.count {
  margin: 20px 0 10px;
  font-weight: 600;
}

.results {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.card {
  display: grid;
  grid-template-columns: 112px 1fr;
  gap: 12px;
  padding: 10px;
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.mat {
  display: block;
  align-self: start;
  aspect-ratio: 4 / 3;
  padding: 4px;
  background: var(--film);
  border-radius: 4px;
}

.body {
  min-width: 0;
}

.body p {
  margin: 0;
}

.fname {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13.5px;
  color: var(--ink-soft);
}

.caption {
  margin-top: 4px;
  font-size: 15px;
  line-height: 1.5;
}

.caption mark {
  background: var(--pencil);
  color: var(--ink);
  border-radius: 2px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}

.chips button {
  padding: 0 9px;
  border: 1px solid var(--cyan-100);
  border-radius: 999px;
  background: var(--cyan-100);
  color: var(--cyan-800);
  font: 400 13px/1.7 var(--font-body);
  cursor: pointer;
}

.chips button:hover,
.chips button.on {
  background: var(--pencil);
  border-color: var(--pencil);
  color: var(--ink);
}

@media (max-width: 520px) {
  .card {
    grid-template-columns: 1fr;
  }
  .mat {
    max-width: 200px;
  }
}
</style>