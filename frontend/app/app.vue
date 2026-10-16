<template>
  <div class="shell">
    <aside class="rail">
      <div class="brand">
        <svg width="28" height="28" viewBox="0 0 28 28" aria-hidden="true">
          <rect x="2" y="5" width="24" height="18" rx="3" fill="#1b5b88" stroke="#d9e7f0" stroke-width="1.5" />
          <circle cx="14" cy="14" r="4.5" fill="none" stroke="#f1be2d" stroke-width="2" />
        </svg>
        Photo Backup
      </div>

      <nav class="nav" aria-label="เมนูหลัก">
        <NuxtLink to="/">ย้ายรูป</NuxtLink>
        <NuxtLink to="/destination">รูปในปลายทาง</NuxtLink>
        <NuxtLink to="/search">ค้นหารูป</NuxtLink>
        <NuxtLink to="/history">ประวัติ</NuxtLink>
      </nav>

      <dl class="rail-folders">
        <dt>ต้นทาง</dt>
        <dd :title="source">{{ source || 'ยังไม่ได้เลือก' }}</dd>
        <dt>ปลายทาง</dt>
        <dd :title="dest">{{ dest || 'ยังไม่ได้เลือก' }}</dd>
      </dl>
    </aside>

    <main class="stage">
      <div class="stage-inner">
        <p v-if="!wails" class="notice notice-warn" role="alert">
          หน้านี้เปิดนอกแอป Wails จึงเรียกใช้งานฝั่ง Go ไม่ได้ ให้รัน <code>wails dev</code> แล้วใช้หน้าต่างโปรแกรมที่เปิดขึ้นมา
        </p>
        <NuxtPage />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
const { source, dest, reload } = useFolders()
const wails = ref(true)

onMounted(async () => {
  wails.value = inWails()
  if (!wails.value) return
  try {
    await reload()
  } catch {
    // ถ้าอ่านค่าที่จำไว้ไม่ได้ ก็เริ่มต้นแบบยังไม่ได้เลือกโฟลเดอร์
  }
})
</script>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 232px 1fr;
  height: 100vh;
}

.rail {
  background: var(--cyan-900);
  color: #e6eef4;
  display: flex;
  flex-direction: column;
  gap: 26px;
  padding: 22px 14px;
  overflow: hidden;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 6px;
  font: 600 18px/1.2 var(--font-head);
  color: #fff;
}

.brand svg {
  flex: none;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav a {
  position: relative;
  display: block;
  padding: 9px 12px 9px 16px;
  border-radius: 8px;
  color: #c9d8e3;
  font-weight: 600;
  text-decoration: none;
}

.nav a:hover {
  background: rgba(255, 255, 255, 0.07);
  color: #fff;
}

.nav a.router-link-exact-active {
  background: var(--cyan-800);
  color: #fff;
}

.nav a.router-link-exact-active::before {
  content: "";
  position: absolute;
  left: 0;
  top: 9px;
  bottom: 9px;
  width: 4px;
  border-radius: 2px;
  background: var(--pencil);
}

.nav a:focus-visible,
.rail .btn:focus-visible {
  outline-color: var(--pencil);
}

.rail-folders {
  margin: auto 0 0;
  padding: 0 6px;
  font-size: 13.5px;
  line-height: 1.5;
  color: #a9bdcb;
}

.rail-folders dt {
  margin-top: 12px;
  font-weight: 600;
  color: #e6eef4;
}

.rail-folders dd {
  margin: 2px 0 0;
  overflow: hidden;
  word-break: break-all;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.stage {
  overflow-y: auto;
  padding: 30px 36px 0;
}

.stage-inner {
  max-width: 1120px;
  margin: 0 auto;
}

@media (max-width: 760px) {
  .shell {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }
  .rail {
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px 18px;
    padding: 12px 14px;
  }
  .nav {
    flex-direction: row;
  }
  .rail-folders {
    display: none;
  }
  .stage {
    padding: 22px 16px 0;
  }
}
</style>