<template>
  <section>
    <header class="pagehead">
      <h1>รูปในปลายทาง</h1>
      <p class="lede">รูปที่ย้ายมาเก็บไว้แล้ว ตรวจว่าไฟล์ยังครบตามประวัติ หรือลบรูปที่ไม่ต้องการ</p>
    </header>

    <p v-if="!dest" class="empty-state">
      ยังไม่ได้เลือกโฟลเดอร์ปลายทาง <NuxtLink to="/">ไปเลือกที่หน้าย้ายรูป</NuxtLink>
    </p>

    <template v-else>
      <div class="scope">
        <p class="path" :title="dest">{{ dest }}</p>
        <button type="button" class="btn btn-quiet" :disabled="checking || busy" @click="check">
          {{ checking ? 'กำลังตรวจ' : 'ตรวจความถูกต้อง' }}
        </button>
        <button type="button" class="btn btn-text" :disabled="busy" @click="load">โหลดใหม่</button>
      </div>

      <p v-if="error" class="notice notice-danger" role="alert">{{ error }}</p>

      <div v-if="integrity" class="panel">
        <h2>ผลตรวจความถูกต้อง</h2>
        <template v-if="!integrity.length">
          <p>ยังไม่มีประวัติการสำรองของปลายทางนี้ จึงไม่มีอะไรให้ตรวจ</p>
        </template>
        <template v-else>
          <p>เทียบรายการในฐานข้อมูลกับไฟล์จริงในโฟลเดอร์ปลายทาง</p>
          <span class="stat good"><strong>{{ integrity.length - missing.length }}</strong> ครบ</span>
          <span class="stat" :class="{ bad: missing.length }"><strong>{{ missing.length }}</strong> ไม่พบไฟล์</span>
          <ul v-if="missing.length" class="missing-list">
            <li v-for="m in missing" :key="m.filename">{{ m.filename }}</li>
          </ul>
          <p v-if="missing.length">
            ถ้าไฟล์เหล่านี้ถูกย้ายหรือลบนอกโปรแกรม ล้างประวัติได้ที่หน้า <NuxtLink to="/history">ประวัติ</NuxtLink>
          </p>
        </template>
      </div>

      <div v-if="result" class="notice" :class="result.failed ? 'notice-warn' : 'notice-ok'" role="status">
        <button type="button" class="btn btn-text dismiss" @click="result = null">ปิด</button>
        ลบแล้ว {{ result.deleted }} รูป ไม่สำเร็จ {{ result.failed }} รูป
        <details v-if="problems.length">
          <summary>ดูรายละเอียด ({{ problems.length }})</summary>
          <ul>
            <li v-for="p in problems" :key="p.name">{{ p.name }}: {{ p.message }}</li>
          </ul>
        </details>
      </div>

      <section class="listing">
        <div class="listing-head">
          <h2>รูปในปลายทาง ({{ files.length }})</h2>
          <div class="tools">
            <button type="button" class="btn btn-text" :disabled="!files.length" @click="selected = files.map((f) => f.name)">
              เลือกทั้งหมด
            </button>
            <button type="button" class="btn btn-text" :disabled="!selected.length" @click="selected = []">
              ล้างการเลือก
            </button>
          </div>
        </div>

        <PhotoGrid v-if="files.length" v-model="selected" :items="files" kind="dest" />
        <p v-else class="empty-state">ยังไม่มีรูปในโฟลเดอร์ปลายทาง</p>
      </section>

      <div class="actionbar">
        <p class="status">
          <span v-if="busy">กำลังลบรูป</span>
          <span v-else>เลือกแล้ว {{ selected.length }} จาก {{ files.length }} รูป</span>
        </p>
        <button type="button" class="btn btn-danger-quiet" :disabled="busy || !files.length" @click="confirmMode = 'all'">
          ลบทั้งหมด
        </button>
        <button type="button" class="btn btn-danger" :disabled="busy || !selected.length" @click="confirmMode = 'selected'">
          ลบที่เลือก
        </button>
      </div>
    </template>

    <ConfirmDialog
      :open="confirmMode !== null"
      :title="confirmMode === 'all' ? `ลบรูปทั้งหมด ${files.length} รูป?` : `ลบรูปที่เลือก ${selected.length} รูป?`"
      message="ไฟล์จะถูกลบออกจากโฟลเดอร์ปลายทางและลบประวัติในฐานข้อมูลด้วย ย้อนกลับไม่ได้"
      confirm-text="ลบรูป"
      danger
      @confirm="runDelete(confirmMode!)"
      @cancel="confirmMode = null"
    />
  </section>
</template>

<script setup lang="ts">
import { ListDest, CheckIntegrity, Delete, DeleteAll } from '~~/wailsjs/go/service/PhotoService'
import type { service } from '~~/wailsjs/go/models'

useHead({ title: 'รูปในปลายทาง | Photo Backup' })

const { dest } = useFolders()

const files = ref<service.FileInfo[]>([])
const selected = ref<string[]>([])
const integrity = ref<service.IntegrityItem[] | null>(null)
const result = ref<service.DeleteResult | null>(null)
const error = ref('')
const busy = ref(false)
const checking = ref(false)
const confirmMode = ref<'selected' | 'all' | null>(null)

const missing = computed(() => (integrity.value ?? []).filter((i) => !i.ok))
const problems = computed(() => (result.value?.items ?? []).filter((i) => i.status !== 'deleted' || i.message))

async function load() {
  error.value = ''
  if (!dest.value) {
    files.value = []
    return
  }
  try {
    files.value = (await ListDest()) ?? []
    const names = new Set(files.value.map((f) => f.name))
    selected.value = selected.value.filter((n) => names.has(n))
  } catch (e) {
    files.value = []
    error.value = errText(e)
  }
}
watch(dest, () => {
  integrity.value = null
  load()
}, { immediate: true })

async function check() {
  checking.value = true
  error.value = ''
  try {
    integrity.value = (await CheckIntegrity()) ?? []
  } catch (e) {
    error.value = errText(e)
  } finally {
    checking.value = false
  }
}

async function runDelete(mode: 'selected' | 'all') {
  confirmMode.value = null
  busy.value = true
  error.value = ''
  result.value = null
  try {
    result.value = mode === 'all' ? await DeleteAll() : await Delete(selected.value)
    selected.value = []
    await load()
    // ถ้าเคยตรวจไว้แล้ว ให้ตรวจซ้ำเพื่อให้ผลตรงกับสถานะล่าสุด
    if (integrity.value) await check()
  } catch (e) {
    error.value = errText(e)
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.missing-list {
  margin: 12px 0 0;
  padding-left: 20px;
  max-height: 200px;
  overflow-y: auto;
  color: var(--danger);
}
</style>
