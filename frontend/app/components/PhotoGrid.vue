<template>
  <div>
    <ul class="sheet">
      <li v-for="f in visible" :key="f.name">
        <button
          type="button"
          class="frame"
          :class="{ done: kind === 'source' && f.inDest }"
          role="checkbox"
          :aria-checked="isSelected(f.name)"
          :title="f.name"
          @click="toggle(f.name)"
        >
          <span class="mat">
            <PhotoThumb :kind="kind" :name="f.name" />
            <span class="tick" aria-hidden="true">
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path d="M2.5 6.3 5 8.8l4.6-5.2" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </span>
            <span v-if="kind === 'source' && f.inDest" class="chip">มีในปลายทางแล้ว</span>
          </span>
          <span class="cap">
            <span class="cap-name">{{ f.name }}</span>
            <span class="cap-size">{{ formatSize(f.size) }}</span>
          </span>
        </button>
      </li>
    </ul>
    <p v-if="items.length > limit" class="more">
      <button type="button" class="btn btn-quiet" @click="limit += 60">
        แสดงเพิ่ม (อีก {{ items.length - limit }} รูป)
      </button>
    </p>
  </div>
</template>

<script setup lang="ts">
import type { service } from '~~/wailsjs/go/models'

const props = defineProps<{
  items: service.FileInfo[]
  kind: 'source' | 'dest'
  modelValue: string[]
}>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()

const limit = ref(60)

const visible = computed(() => props.items.slice(0, limit.value))
const chosen = computed(() => new Set(props.modelValue))

const isSelected = (name: string) => chosen.value.has(name)

function toggle(name: string) {
  const next = new Set(chosen.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  emit('update:modelValue', [...next])
}
</script>

<style scoped>
.sheet {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(148px, 1fr));
  gap: 18px 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.frame {
  display: block;
  width: 100%;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.mat {
  position: relative;
  display: block;
  aspect-ratio: 4 / 3;
  padding: 6px;
  background: var(--film);
  border-radius: 4px;
}

.frame.done :deep(img) {
  opacity: 0.4;
}

.chip {
  position: absolute;
  left: 10px;
  bottom: 10px;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--paper);
  color: var(--ink);
  font-size: 12.5px;
  font-weight: 600;
  line-height: 1.6;
}

.tick {
  position: absolute;
  top: 10px;
  left: 10px;
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border: 2px solid rgba(255, 255, 255, 0.85);
  border-radius: 50%;
  background: rgba(16, 23, 28, 0.35);
  color: transparent;
}

.frame[aria-checked="true"] .tick {
  border-color: var(--pencil);
  background: var(--pencil);
  color: var(--ink);
}

/* วงดินสอเขียนแผ่นฟิล์ม: ขอบโค้งไม่เท่ากันให้เหมือนวาดด้วยมือ */
.frame[aria-checked="true"] .mat {
  outline: 3px solid var(--pencil);
  outline-offset: 3px;
  border-radius: 10px 12px 9px 13px / 12px 9px 13px 10px;
}

.frame:focus-visible {
  outline: none;
}

.frame:focus-visible .mat {
  outline: 3px solid var(--cyan-600);
  outline-offset: 3px;
}

.frame[aria-checked="true"]:focus-visible .mat {
  outline-color: var(--pencil);
  box-shadow: 0 0 0 6px var(--cyan-600);
}

.cap {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-top: 6px;
  font-size: 13.5px;
  line-height: 1.4;
}

.cap-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cap-size {
  flex: none;
  color: var(--ink-soft);
}
</style>
