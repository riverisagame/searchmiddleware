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
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Odometer, Refresh, Timer, Document, Checked, Setting, Link, Bell, User, Search, Moon, Sunny } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const isDark = ref(document.documentElement.classList.contains('dark'))

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
</script>

<style scoped>
.aside {
  background: linear-gradient(180deg, #14203c 0%, #0e1730 100%);
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 20px 20px 16px;
}
.logo-icon { font-size: 22px; }
.logo-text { color: #fff; font-size: 17px; font-weight: 700; letter-spacing: 0.5px; }

.side-menu { border-right: none; padding: 4px 10px; }
.side-menu .el-menu-item {
  border-radius: 8px;
  margin-bottom: 4px;
  height: 44px;
}
.side-menu .el-menu-item:hover { background: rgba(255, 255, 255, 0.06); }
.side-menu .el-menu-item.is-active {
  background: linear-gradient(90deg, var(--el-color-primary), #5a86f8);
  box-shadow: 0 2px 8px rgba(59, 108, 246, 0.35);
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
.main { background: var(--el-bg-color-page); padding: 20px; }
html.dark .header { border-bottom-color: #2c2c2d; }
html.dark .aside { background: linear-gradient(180deg, #1a1c1f 0%, #121316 100%); }
</style>
