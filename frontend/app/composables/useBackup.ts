import { GetState, GetThumbnail } from '~~/wailsjs/go/service/PhotoService'

/** โฟลเดอร์ต้นทาง/ปลายทางที่ใช้อยู่ ใช้ร่วมกันทุกหน้า */
export function useFolders() {
  const source = useState<string>('folders-source', () => '')
  const dest = useState<string>('folders-dest', () => '')
  const ready = useState<boolean>('folders-ready', () => false)

  /** ดึงค่าที่ backend จำไว้ (เรียกตอนเปิดแอป และหลังรีเซ็ตฐานข้อมูล) */
  async function reload() {
    const st = await GetState()
    source.value = st.source
    dest.value = st.dest
    ready.value = true
  }

  return { source, dest, ready, reload }
}

/** true เมื่อเปิดในแอป Wails (มี backend ให้เรียก) false เมื่อเปิดหน้าเว็บในเบราว์เซอร์ธรรมดา */
export function inWails(): boolean {
  return typeof window !== 'undefined' && !!(window as any).go?.service?.PhotoService
}

const thumbCache = new Map<string, Promise<string>>()

/**
 * ขอรูปตัวอย่าง (data URL) จาก backend โดยจำผลไว้ จึงไม่ต้องย่อซ้ำเมื่อสลับหน้าไปมา
 * folder ใช้เป็นส่วนหนึ่งของคีย์ เพราะโฟลเดอร์ต่างกันอาจมีไฟล์ชื่อเดียวกัน
 */
export function loadThumb(kind: 'source' | 'dest', folder: string, name: string): Promise<string> {
  const key = `${kind}|${folder}|${name}`
  let hit = thumbCache.get(key)
  if (!hit) {
    hit = GetThumbnail(kind, name)
    thumbCache.set(key, hit)
    hit.catch(() => thumbCache.delete(key))
    if (thumbCache.size > 500) thumbCache.delete(thumbCache.keys().next().value!) // เก็บไม่เกิน 500 รูป
  }
  return hit
}

export function errText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return String(e)
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

export function formatDate(value: unknown): string {
  const d = new Date(value as string)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString('th-TH', { dateStyle: 'medium', timeStyle: 'short' })
}
