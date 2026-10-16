<template>
  <div v-if="open" class="scrim" @keydown.esc="emit('cancel')" @click.self="emit('cancel')">
    <div class="dialog" role="alertdialog" aria-modal="true" :aria-labelledby="`${uid}-t`" :aria-describedby="`${uid}-d`">
      <h2 :id="`${uid}-t`">{{ title }}</h2>
      <p :id="`${uid}-d`">{{ message }}</p>
      <div class="row">
        <button ref="cancelBtn" type="button" class="btn btn-quiet" @click="emit('cancel')">ยกเลิก</button>
        <button type="button" class="btn" :class="danger ? 'btn-danger' : 'btn-primary'" @click="emit('confirm')">
          {{ confirmText }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  open: boolean
  title: string
  message: string
  confirmText: string
  danger?: boolean
}>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()

const uid = useId()
const cancelBtn = ref<HTMLButtonElement | null>(null)

// ให้โฟกัสที่ปุ่ม "ยกเลิก" ก่อนเสมอ กันการกด Enter แล้วยืนยันโดยไม่ตั้งใจ
watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen) {
      await nextTick()
      cancelBtn.value?.focus()
    }
  },
)
</script>

<style scoped>
.scrim {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(12, 44, 69, 0.55);
  animation: fade 0.12s ease-out;
}

.dialog {
  width: 100%;
  max-width: 460px;
  padding: 22px 24px;
  background: var(--card);
  border-radius: 12px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.3);
}

.dialog h2 {
  margin-bottom: 8px;
  font-size: 19px;
}

.dialog p {
  margin: 0 0 20px;
  color: var(--ink-soft);
}

.dialog .row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@keyframes fade {
  from {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .scrim {
    animation: none;
  }
}
</style>
