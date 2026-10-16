<template>
  <section>
    <header class="pagehead">
      <h1>ย้ายรูปไปสำรอง</h1>
      <p class="lede">
        เลือกโฟลเดอร์ที่มีรูป เช่น แฟลชไดรฟ์หรือโทรศัพท์ แล้วเลือกที่เก็บรูปบนเครื่องนี้
        รูปที่ย้ายสำเร็จจะถูกลบออกจากต้นทาง
      </p>
    </header>

    <div class="route">
      <div class="station">
        <h2>ต้นทาง</h2>
        <p class="path" :class="{ empty: !source }" :title="source">{{ source || 'ยังไม่ได้เลือกโฟลเดอร์' }}</p>
        <p v-if="source && !error" class="meta">พบ {{ files.length }} รูป</p>
        <button type="button" class="btn btn-quiet" :disabled="busy" @click="pickSource">
          {{ source ? 'เปลี่ยนโฟลเดอร์' : 'เลือกโฟลเดอร์ต้นทาง' }}
        </button>
      </div>

      <svg class="route-line" viewBox="0 0 48 20" fill="none" aria-hidden="true">
        <path d="M2 10h40M35 3l8 7-8 7" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
      </svg>

      <div class="station">
        <h2>ปลายทาง</h2>
        <p class="path" :class="{ empty: !dest }" :title="dest">{{ dest || 'ยังไม่ได้เลือกโฟลเดอร์' }}</p>
        <p v-if="dest" class="meta">รูปที่ย้ายแล้วจะอยู่ที่นี่</p>
        <button type="button" class="btn btn-quiet" :disabled="busy" @click="pickDest">
          {{ dest ? 'เปลี่ยนโฟลเดอร์' : 'เลือกโฟลเดอร์ปลายทาง' }}
        </button>
      </div>
    </div>

    <p v-if="error" class="notice notice-danger" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice notice-warn" role="status">{{ notice }}</p>

    <p v-if="destCheck && destCheck.total > 0 && destCheck.missing === 0" class="notice notice-ok" role="status">
      ตรวจปลายทางแล้ว รูปที่เคยสำรองไว้อยู่ครบ {{ destCheck.total }} รูป
    </p>
    <p v-else-if="destCheck && destCheck.missing > 0" class="notice notice-warn" role="status">
      ไม่พบไฟล์ {{ destCheck.missing }} จาก {{ destCheck.total }} รูปที่บันทึกไว้ในประวัติ
      <NuxtLink to="/destination">ดูรายละเอียด</NuxtLink>
    </p>

    <div v-if="result" class="notice" :class="result.failed ? 'notice-warn' : 'notice-ok'" role="status">
      <button type="button" class="btn btn-text dismiss" @click="result = null">ปิด</button>
      ย้ายแล้ว {{ result.moved }} รูป ข้าม {{ result.skipped }} รูป ไม่สำเร็จ {{ result.failed }} รูป
      ใช้เวลา {{ result.seconds.toFixed(3) }} วินาที
      <details v-if="problems.length">
        <summary>ดูรูปที่ข้ามหรือไม่สำเร็จ ({{ problems.length }})</summary>
        <ul>
          <li v-for="p in problems" :key="p.name">{{ p.name }}: {{ p.message }}</li>
        </ul>
      </details>
    </div>

    <section v-if="source && !error" class="listing">
      <div class="listing-head">
        <h2>รูปในต้นทาง</h2>
        <div class="tools">
          <label class="check"><input v-model="hideDone" type="checkbox" /> ซ่อนรูปที่มีในปลายทางแล้ว</label>
          <button type="button" class="btn btn-text" :disabled="!shown.length" @click="selected = shown.map((f) => f.name)">
            เลือกทั้งหมด
          </button>
          <button type="button" class="btn btn-text" :disabled="!selected.length" @click="selected = []">
            ล้างการเลือก
          </button>
          <button type="button" class="btn btn-text" :disabled="busy" @click="loadSource">โหลดใหม่</button>
        </div>
      </div>

      <PhotoGrid v-if="shown.length" v-model="selected" :items="shown" kind="source" />
      <p v-else-if="files.length" class="empty-state">รูปทั้งหมดมีในปลายทางแล้ว</p>
      <p v-else class="empty-state">ไม่พบไฟล์รูปในโฟลเดอร์นี้ (รองรับ JPG, PNG, GIF, WebP, BMP, HEIC)</p>
    </section>
    <p v-else-if="!source" class="empty-state">เลือกโฟลเดอร์ต้นทางเพื่อดูรูปที่จะย้าย</p>
    <p v-else class="empty-state">
      อ่านโฟลเดอร์ต้นทางไม่ได้ ถ้าเป็นแฟลชไดรฟ์ ให้เสียบเครื่องให้เรียบร้อยแล้วกด
      <button type="button" class="btn btn-text" @click="loadSource">โหลดใหม่</button>
    </p>

    <div v-if="source" class="actionbar">
      <div class="status">
        <template v-if="busy">
          <progress v-if="progress" :value="progress.done" :max="progress.total" aria-label="ความคืบหน้าการย้ายรูป" />
          <progress v-else aria-label="กำลังย้ายรูป" />
          <span class="progress-name">
            <template v-if="progress">กำลังย้าย {{ progress.done }} จาก {{ progress.total }} รูป: {{ progress.name }}</template>
            <template v-else>กำลังเตรียมย้ายรูป</template>
          </span>
        </template>
        <span v-else-if="!dest" class="hint">เลือกโฟลเดอร์ปลายทางก่อนจึงจะย้ายรูปได้</span>
        <span v-else>เลือกแล้ว {{ selected.length }} จาก {{ files.length }} รูป</span>
      </div>
      <button type="button" class="btn btn-quiet" :disabled="!canMove || !files.length" @click="confirmMode = 'all'">
        ย้ายทั้งหมด
      </button>
      <button type="button" class="btn btn-primary" :disabled="!canMove || !selected.length" @click="confirmMode = 'selected'">
        ย้ายที่เลือก
      </button>
    </div>

    <ConfirmDialog
      :open="confirmMode !== null"
      :title="confirmMode === 'all' ? `ย้ายรูปทั้งหมด ${files.length} รูป?` : `ย้ายรูปที่เลือก ${selected.length} รูป?`"
      :message="`รูปจะถูกย้ายไปที่ ${dest} และลบออกจากต้นทางเมื่อย้ายสำเร็จ รูปที่มีชื่อซ้ำในปลายทางจะถูกข้ามและไม่ถูกลบ`"
      confirm-text="ย้ายรูป"
      @confirm="runMove(confirmMode!)"
      @cancel="confirmMode = null"
    />
  </section>
</template>

<script setup lang="ts">
import { SelectFolder, SetSource, SetDest, ListSource, Move, MoveAll } from '~~/wailsjs/go/service/PhotoService'
import { EventsOn } from '~~/wailsjs/runtime/runtime'
import type { service } from '~~/wailsjs/go/models'

useHead({ title: 'ย้ายรูป | Photo Backup' })

const { source, dest } = useFolders()

const files = ref<service.FileInfo[]>([])
const selected = ref<string[]>([])
const hideDone = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const destCheck = ref<{ total: number; missing: number } | null>(null)
const result = ref<service.MoveResult | null>(null)
const progress = ref<{ done: number; total: number; name: string } | null>(null)
const confirmMode = ref<'selected' | 'all' | null>(null)

const shown = computed(() => (hideDone.value ? files.value.filter((f) => !f.inDest) : files.value))
const canMove = computed(() => !!dest.value && !busy.value)
const problems = computed(() => (result.value?.items ?? []).filter((i) => i.status !== 'moved' || i.message))

async function loadSource() {
  error.value = ''
  if (!source.value) {
    files.value = []
    selected.value = []
    return
  }
  try {
    files.value = (await ListSource()) ?? []
    const names = new Set(files.value.map((f) => f.name))
    selected.value = selected.value.filter((n) => names.has(n))
  } catch (e) {
    files.value = []
    error.value = errText(e)
  }
}

// โหลดรายการใหม่ทุกครั้งที่เปลี่ยนต้นทางหรือปลายทาง (ปลายทางมีผลต่อป้าย "มีในปลายทางแล้ว")
watch([source, dest], loadSource, { immediate: true })

async function pickSource() {
  error.value = ''
  notice.value = ''
  try {
    const dir = await SelectFolder('เลือกโฟลเดอร์ต้นทาง (แฟลชไดรฟ์หรือโทรศัพท์)')
    if (!dir) return
    const info = await SetSource(dir)
    notice.value = info.warning
    result.value = null
    const changed = source.value !== info.path
    source.value = info.path
    if (!changed) await loadSource()
  } catch (e) {
    error.value = errText(e)
  }
}

async function pickDest() {
  error.value = ''
  notice.value = ''
  try {
    const dir = await SelectFolder('เลือกโฟลเดอร์ปลายทาง (ที่เก็บรูปสำรอง)')
    if (!dir) return
    const info = await SetDest(dir)
    notice.value = info.warning
    result.value = null
    // ตั้งปลายทางแล้ว backend ตรวจว่ารูปที่เคยสำรองไว้ยังอยู่ครบหรือไม่ให้ทันที
    const items = info.integrity ?? []
    destCheck.value = { total: items.length, missing: items.filter((i) => !i.ok).length }
    const changed = dest.value !== info.path
    dest.value = info.path
    if (!changed) await loadSource()
  } catch (e) {
    error.value = errText(e)
  }
}

async function runMove(mode: 'selected' | 'all') {
  confirmMode.value = null
  busy.value = true
  error.value = ''
  notice.value = ''
  result.value = null
  progress.value = null
  try {
    result.value = mode === 'all' ? await MoveAll() : await Move(selected.value)
    selected.value = []
  } catch (e) {
    error.value = errText(e)
  } finally {
    busy.value = false
    progress.value = null
    await loadSource()
  }
}

let stopProgress: (() => void) | undefined
onMounted(() => {
  if (inWails()) {
    stopProgress = EventsOn('move:progress', (p: { done: number; total: number; name: string }) => {
      progress.value = p
    })
  }
})
onBeforeUnmount(() => stopProgress?.())
</script>

<style scoped>
.route {
  display: grid;
  grid-template-columns: 1fr 64px 1fr;
  align-items: center;
  margin: 22px 0 10px;
}

.station {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-height: 150px;
  padding: 16px 18px;
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.station h2 {
  font-size: 16px;
}

.station .btn {
  margin-top: auto;
}

.route-line {
  justify-self: center;
  width: 48px;
  height: 20px;
  color: var(--cyan-600);
}

@media (max-width: 760px) {
  .route {
    grid-template-columns: 1fr;
    gap: 10px;
  }
  .route-line {
    display: none;
  }
}

progress {
  appearance: none;
  width: 100%;
  height: 8px;
  border: 0;
  border-radius: 4px;
  background: var(--cyan-100);
}

progress::-webkit-progress-bar {
  background: var(--cyan-100);
  border-radius: 4px;
}

progress::-webkit-progress-value {
  background: var(--cyan-600);
  border-radius: 4px;
  transition: width 0.15s;
}

progress::-moz-progress-bar {
  background: var(--cyan-600);
  border-radius: 4px;
}

@media (prefers-reduced-motion: reduce) {
  progress::-webkit-progress-value {
    transition: none;
  }
}
</style>
