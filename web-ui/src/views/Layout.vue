<template>
  <el-container style="height: 100vh">
    <el-aside width="var(--app-sidebar-width)" class="aside">
      <div class="logo">
        <div class="logo-icon">🔍</div>
        <div class="logo-text">搜索中间件</div>
      </div>
      <el-menu :default-active="$route.path" router class="side-menu" background-color="transparent"
        text-color="#a7b0c2" active-text-color="#fff">
        <el-menu-item index="/dashboard"><el-icon><Odometer /></el-icon><span>仪表盘</span></el-menu-item>
        <el-menu-item index="/search-test"><el-icon><Search /></el-icon><span>搜索测试</span></el-menu-item>
        <el-menu-item index="/sync"><el-icon><Refresh /></el-icon><span>同步中心</span></el-menu-item>
        <el-menu-item index="/schedules"><el-icon><Timer /></el-icon><span>定时任务</span></el-menu-item>
        <el-menu-item index="/logs"><el-icon><Document /></el-icon><span>日志中心</span></el-menu-item>
        <el-menu-item index="/reconcile"><el-icon><Checked /></el-icon><span>对账中心</span></el-menu-item>
        <el-menu-item index="/indexes"><el-icon><Setting /></el-icon><span>索引配置</span></el-menu-item>
        <el-menu-item index="/synonyms"><el-icon><Link /></el-icon><span>同义词</span></el-menu-item>
        <el-menu-item index="/alerts"><el-icon><Bell /></el-icon><span>告警中心</span></el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/users"><el-icon><User /></el-icon><span>用户管理</span></el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="page-title">{{ $route.meta.title || '' }}</span>
        <div class="header-right">
          <!-- 系统状态指示 -->
          <el-tooltip :content="`Zinc ${sysHealth.zinc ? '正常' : '异常'} · 每 30s 检测`" placement="bottom">
            <span class="sys-status" @click="checkHealth">
              <i :class="['sys-dot', sysHealth.zinc ? 'ok' : 'bad']"></i>
              <span class="sys-label">Zinc</span>
            </span>
          </el-tooltip>
          <el-tooltip :content="isDark ? '切换到亮色' : '切换到暗色'">
            <el-button circle size="small" @click="toggleTheme">
              <el-icon><Moon v-if="!isDark" /><Sunny v-else /></el-icon>
            </el-button>
          </el-tooltip>
          <el-dropdown @command="onCommand">
            <span class="user-chip">
              <el-avatar :size="26" style="background: var(--el-color-primary)">{{ (auth.user?.username || 'A')[0].toUpperCase() }}</el-avatar>
              <span class="user-name">{{ auth.user?.username }}<em class="user-role">{{ auth.user?.role }}</em></span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>

      <!-- Cmd+K 命令面板 -->
      <div v-if="cmdOpen" class="cmd-overlay" @click.self="cmdOpen = false">
        <div class="cmd-panel">
          <div class="cmd-input-row">
            <el-icon style="color: var(--el-text-color-secondary)"><Search /></el-icon>
            <input v-model="cmdQuery" ref="cmdInput" class="cmd-input" placeholder="输入页面名或索引名搜索… 按 Esc 关闭" @keydown.esc="cmdOpen = false" />
          </div>
          <div class="cmd-list">
            <div v-for="item in cmdResults" :key="item.key" class="cmd-item" @click="cmdGo(item)">
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
              <em class="cmd-type">{{ item.type }}</em>
            </div>
            <div v-if="!cmdResults.length" class="cmd-empty">无匹配结果</div>
          </div>
        </div>
      </div>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Odometer, Refresh, Timer, Document, Checked, Setting, Link, Bell, User, Search, Moon, Sunny } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { api } from '../api'

const router = useRouter()
const auth = useAuthStore()
const isDark = ref(document.documentElement.classList.contains('dark'))

// 系统健康指示
const sysHealth = ref({ zinc: false })
let healthTimer = null
async function checkHealth() {
  try {
    const r = await fetch('/health')
    const j = await r.json()
    sysHealth.value = { zinc: !!j.zinc }
  } catch {
    sysHealth.value = { zinc: false }
  }
}

// Cmd+K 命令面板
const cmdOpen = ref(false)
const cmdQuery = ref('')
const cmdInput = ref(null)
const cmdIndexes = ref([])

const PAGES = [
  { key: 'dashboard', label: '仪表盘', icon: Odometer, path: '/dashboard', type: '页面' },
  { key: 'search-test', label: '搜索测试', icon: Search, path: '/search-test', type: '页面' },
  { key: 'sync', label: '同步中心', icon: Refresh, path: '/sync', type: '页面' },
  { key: 'schedules', label: '定时任务', icon: Timer, path: '/schedules', type: '页面' },
  { key: 'logs', label: '日志中心', icon: Document, path: '/logs', type: '页面' },
  { key: 'reconcile', label: '对账中心', icon: Checked, path: '/reconcile', type: '页面' },
  { key: 'indexes', label: '索引配置', icon: Setting, path: '/indexes', type: '页面' },
  { key: 'synonyms', label: '同义词', icon: Link, path: '/synonyms', type: '页面' },
  { key: 'alerts', label: '告警中心', icon: Bell, path: '/alerts', type: '页面' },
  { key: 'users', label: '用户管理', icon: User, path: '/users', type: '页面', admin: true },
]

const cmdResults = computed(() => {
  const q = cmdQuery.value.trim().toLowerCase()
  const pages = PAGES.filter((p) => !p.admin || auth.isAdmin)
  const items = pages
    .filter((p) => p.label.includes(q) || p.key.includes(q))
    .map((p) => ({ ...p, type: '页面' }))
  if (q) {
    for (const i of cmdIndexes.value) {
      if (String(i).toLowerCase().includes(q)) {
        items.push({ key: 'idx-' + i, label: i, icon: Setting, path: '/indexes', type: '索引' })
      }
    }
  }
  return items.slice(0, 12)
})

async function cmdGo(item) {
  cmdOpen.value = false
  cmdQuery.value = ''
  if (item.type === '索引') {
    router.push(item.path)
  } else {
    router.push(item.path)
  }
}

function onKeydown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    cmdOpen.value = !cmdOpen.value
    if (cmdOpen.value) {
      cmdQuery.value = ''
      nextTick(() => cmdInput.value?.focus())
    }
  }
}

onMounted(() => {
  const saved = localStorage.getItem('sm_theme')
  if (saved === 'dark') document.documentElement.classList.add('dark')
  if (saved === 'light') document.documentElement.classList.remove('dark')
  isDark.value = document.documentElement.classList.contains('dark')
})

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('sm_theme', isDark.value ? 'dark' : 'light')
}

function onCommand(cmd) {
  if (cmd === 'logout') {
    auth.logout()
    router.push('/login')
  }
}

onMounted(() => {
  api.listIndexes().then((list) => { cmdIndexes.value = list || [] }).catch(() => {})
  window.addEventListener('keydown', onKeydown)
  checkHealth()
  healthTimer = setInterval(checkHealth, 30000)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  clearInterval(healthTimer)
})
</script>

<style scoped>
.aside {
  background: linear-gradient(180deg, #101a33 0%, #0b1226 100%);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
  border-right: 1px solid rgba(34, 211, 238, 0.08);
}
.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 22px 20px 18px;
}
.logo-icon {
  font-size: 22px;
  filter: drop-shadow(0 0 6px rgba(34, 211, 238, 0.5));
}
.logo-text {
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.5px;
  font-family: var(--app-font-mono);
}

.side-menu { border-right: none; padding: 6px 12px; }
.side-menu .el-menu-item {
  border-radius: 10px;
  margin-bottom: 6px;
  height: 46px;
  font-size: 14px;
}
.side-menu .el-menu-item:hover { background: rgba(255, 255, 255, 0.06); }
.side-menu .el-menu-item.is-active {
  background: linear-gradient(90deg, rgba(34, 211, 238, 0.2), rgba(34, 211, 238, 0.05));
  box-shadow: inset 0 0 0 1px rgba(34, 211, 238, 0.35), 0 0 12px rgba(34, 211, 238, 0.12);
  color: #7ceaf7;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
  height: 56px !important;
  padding: 0 20px;
}
.header-right { display: flex; align-items: center; gap: 14px; }
.sys-status { display: flex; align-items: center; gap: 6px; cursor: pointer; padding: 4px 10px; border-radius: 20px; background: var(--el-fill-color-lighter); }
.sys-dot { width: 8px; height: 8px; border-radius: 50%; }
.sys-dot.ok { background: #34a853; box-shadow: 0 0 6px rgba(52, 168, 83, 0.5); }
.sys-dot.bad { background: #e5484d; box-shadow: 0 0 6px rgba(229, 72, 77, 0.5); }
.sys-label { font-size: 12px; color: var(--el-text-color-secondary); font-family: var(--app-font-mono); }
.page-title { font-size: 16px; font-weight: 600; }
.user-chip { display: flex; align-items: center; gap: 8px; cursor: pointer; }
.user-name { font-size: 14px; }
.user-role {
  font-style: normal;
  font-size: 11px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 4px;
  padding: 1px 6px;
  margin-left: 6px;
}
.main { background: var(--el-bg-color-page); padding: 24px; }
html.dark .header { border-bottom-color: var(--el-border-color-lighter); }
html.dark .aside { background: linear-gradient(180deg, #141c30 0%, #0e1424 100%); }

/* Cmd+K 命令面板 */
.cmd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 12vh;
  z-index: 3000;
  animation: fade-up 0.15s ease-out;
}
.cmd-panel {
  width: 520px;
  max-width: 90vw;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
  overflow: hidden;
}
.cmd-input-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.cmd-input {
  flex: 1;
  border: none;
  outline: none;
  background: transparent;
  font-size: 15px;
  color: var(--el-text-color-primary);
}
.cmd-list { max-height: 320px; overflow-y: auto; padding: 6px; }
.cmd-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--el-text-color-regular);
}
.cmd-item:hover { background: var(--el-fill-color-light); color: var(--el-text-color-primary); }
.cmd-type {
  margin-left: auto;
  font-style: normal;
  font-size: 11px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color);
  border-radius: 4px;
  padding: 2px 8px;
}
.cmd-empty { padding: 24px; text-align: center; color: var(--el-text-color-secondary); font-size: 13px; }
</style>
