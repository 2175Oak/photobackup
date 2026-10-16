<template>
  <div>
    <h1>Photo Backup</h1>
    <button @click="selectFolder">เลือกโฟลเดอร์ต้นทาง</button>
    <p>โฟลเดอร์ต้นทาง: {{ sourceDir }}</p>
    <button @click="backup" :disabled="!sourceDir">Backup</button>
    <p>{{ message }}</p>
  </div>
</template>

<script setup lang="ts">
import { SelectFolder, Backup } from "../../wailsjs/go/service/PhotoService"

const sourceDir = ref("")
const message = ref("")

async function selectFolder() {
  const dir = await SelectFolder()
  if (dir) sourceDir.value = dir
}

async function backup() {
  try {
    const n = await Backup(sourceDir.value)
    message.value = `Backup สำเร็จ ${n} รูป`
  } catch (e) {
    message.value = "ผิดพลาด: " + e
  }
}
</script>