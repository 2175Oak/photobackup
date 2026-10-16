<template>
  <span ref="root" class="thumb">
    <img v-if="src" :src="src" alt="" decoding="async" />
    <span v-else-if="failed" class="noimg">ดูตัวอย่างไม่ได้</span>
  </span>
</template>

<script setup lang="ts">
const props = defineProps<{
  kind: 'source' | 'dest'
  name: string
}>()

const { source, dest } = useFolders()

const root = ref<HTMLElement | null>(null)
const src = ref('')
const failed = ref(false)

let seen = false
let observer: IntersectionObserver | undefined

const folder = computed(() => (props.kind === 'source' ? source.value : dest.value))

async function load() {
  failed.value = false
  try {
    src.value = await loadThumb(props.kind, folder.value, props.name)
  } catch {
    // เช่น ไฟล์ HEIC หรือ WebP ที่ backend ย่อให้ไม่ได้
    src.value = ''
    failed.value = true
  }
}

// โหลดรูปเมื่อเลื่อนมาใกล้จอเท่านั้น ไม่ย่อรูปทั้งโฟลเดอร์ตั้งแต่เปิดหน้า
onMounted(() => {
  if (typeof IntersectionObserver === 'undefined') {
    seen = true
    load()
    return
  }
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting)) {
        seen = true
        observer?.disconnect()
        load()
      }
    },
    { rootMargin: '240px' },
  )
  if (root.value) observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())

watch(
  () => [props.name, folder.value],
  () => {
    src.value = ''
    if (seen) load()
  },
)
</script>

<style scoped>
.thumb {
  display: block;
  width: 100%;
  height: 100%;
}

.thumb img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 2px;
}

.noimg {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  padding: 6px;
  color: #9fb0ba;
  font-size: 13px;
  text-align: center;
}
</style>
