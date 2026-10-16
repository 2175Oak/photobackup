<template>
  <section>
    <header class="pagehead">
      <h1>ประวัติการสำรอง</h1>
      <p class="lede">รายการรูปที่เคยย้ายมาเก็บ แยกตามโฟลเดอร์ปลายทาง จะไม่ปนกับปลายทางอื่น</p>
    </header>

    <p v-if="error" class="notice notice-danger" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice notice-ok" role="status">{{ notice }}</p>

    <p v-if="!effective" class="empty-state">
      ยังไม่ได้เลือกโฟลเดอร์ปลายทาง <NuxtLink to="/">ไปเลือกที่หน้าย้ายรูป</NuxtLink>
    </p>

    <template v-else>
      <div class="scope">
        <p class="path" :title="effective">{{ effective }}</p>
        <button type="button" class="btn btn-quiet" :disabled="busy" @click="pickScope">ดูปลายทางอื่น</button>
        <button v-if="scope" type="button" class="btn btn-text" @click="scope = ''">กลับไปปลายทางปัจจุบัน</button>
      </div>

      <table v-if="rows.length" class="table">
        <thead>
          <tr>
            <th v-if="!scope" scope="col"><span class="sr-only">รูป</span></th>
            <th scope="col">ชื่อไฟล์</th>
            <th scope="col">สำรองเมื่อ</th>
            <th scope="col">ตำแหน่งไฟล์</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in shown" :key="r.id">
            <td v-if="!scope">
              <span class="thumbbox"><PhotoThumb kind="dest" :name="r.fileName" /></span>
            </td>
            <td>{{ r.fileName }}</td>
            <td>{{ formatDate(r.backedUpAt) }}</td>
            <td class="wrap">{{ r.backupPath }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else-if="!loading" class="empty-state">ยังไม่มีประวัติการสำรองของปลายทางนี้</p>

      <p v-if="rows.length > limit" class="more">
        <button type="button" class="btn btn-quiet" @click="limit += 100">แสดงเพิ่ม (อีก {{ rows.length - limit }} รายการ)</button>
      </p>

      <div class="panel">
        <h2>ล้างประวัติของปลายทางนี้</h2>
        <p>
          ลบเฉพาะรายการในฐานข้อมูล รูปในโฟลเดอร์ปลายทางยังอยู่เหมือนเดิม
          หลังล้างแล้ว รูปชื่อเดิมจะย้ายมาซ้ำได้อีก
        </p>
        <button type="button" class="btn btn-danger-quiet" :disabled="busy || !!scope || !rows.length" @click="confirmMode = 'dest'">
          ล้างประวัติปลายทางนี้
        </button>
        <p v-if="scope" class="hint-inline">ล้างได้เฉพาะปลายทางปัจจุบัน กลับไปปลายทางปัจจุบันก่อน</p>
      </div>
    </template>

    <div class="panel danger">
      <h2>รีเซ็ตฐานข้อมูลทั้งหมด</h2>
      <p>
        ลบประวัติและแท็กของทุกปลายทาง รวมถึงโฟลเดอร์ต้นทาง/ปลายทางที่โปรแกรมจำไว้
        ไม่ลบไฟล์รูปในเครื่อง
      </p>
      <button type="button" class="btn btn-danger" :disabled="busy" @click="confirmMode = 'all'">รีเซ็ตฐานข้อมูล</button>
    </div>

    <ConfirmDialog
      :open="confirmMode !== null"
      :title="confirmMode === 'all' ? 'รีเซ็ตฐานข้อมูลทั้งหมด?' : 'ล้างประวัติของปลายทางนี้?'"
      :message="confirmMode === 'all'
        ? 'ประวัติและค่าที่จำไว้ทั้งหมดจะถูกลบ ย้อนกลับไม่ได้ ส่วนไฟล์รูปในเครื่องจะไม่ถูกลบ'
        : `ลบ ${rows.length} รายการในฐานข้อมูล รูปในโฟลเดอร์ปลายทางจะไม่ถูกลบ`"
      :confirm-text="confirmMode === 'all' ? 'รีเซ็ต' : 'ล้างประวัติ'"
      danger
      @confirm="run(confirmMode!)"
      @cancel="confirmMode = null"
    />
  </section>
</template>

<script setup lang="ts">
import { ListHistory, SelectFolder, CleanDest, CleanAll } from '~~/wailsjs/go/service/PhotoService'
import type { model } from '~~/wailsjs/go/models'

useHead({ title: 'ประวัติ | Photo Backup' })

const { dest, reload } = useFolders()

const scope = ref('') // ว่าง = ปลายทางปัจจุบัน
const rows = ref<model.Photo[]>([])
const limit = ref(100)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const confirmMode = ref<'dest' | 'all' | null>(null)

const effective = computed(() => scope.value || dest.value)
const shown = computed(() => rows.value.slice(0, limit.value))

async function load() {
  error.value = ''
  limit.value = 100
  if (!effective.value) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    rows.value = (await ListHistory(scope.value)) ?? []
  } catch (e) {
    rows.value = []
    error.value = errText(e)
  } finally {
    loading.value = false
  }
}
watch([scope, dest], load, { immediate: true })

async function pickScope() {
  try {
    const dir = await SelectFolder('เลือกปลายทางที่ต้องการดูประวัติ')
    if (dir) scope.value = dir
  } catch (e) {
    error.value = errText(e)
  }
}

async function run(mode: 'dest' | 'all') {
  confirmMode.value = null
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    if (mode === 'dest') {
      const n = await CleanDest()
      notice.value = `ล้างประวัติแล้ว ${n} รายการ รูปในโฟลเดอร์ปลายทางไม่ถูกลบ`
    } else {
      const r = await CleanAll()
      notice.value = `รีเซ็ตแล้ว ลบประวัติ ${r.photos} รายการ แท็ก ${r.tags} รายการ และค่าที่จำไว้ ${r.settings} รายการ`
      scope.value = ''
      await reload()
    }
    await load()
  } catch (e) {
    error.value = errText(e)
  } finally {
    busy.value = false
  }
}
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
  margin: 10px 0 0;
  font-size: 14px;
}

.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 15px;
}

.table th {
  padding: 8px 12px;
  border-bottom: 2px solid var(--ink);
  color: var(--ink-soft);
  font-weight: 600;
  text-align: left;
}

.table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--line);
  vertical-align: middle;
}

.table td.wrap {
  word-break: break-all;
  color: var(--ink-soft);
  font-size: 13.5px;
}

.table .thumbbox {
  display: block;
  width: 56px;
  height: 42px;
  overflow: hidden;
  border-radius: 3px;
  background: var(--film);
}
</style>
