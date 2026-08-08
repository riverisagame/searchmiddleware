<template>
  <el-container style="height: 100vh">
    <el-aside width="210px" class="aside">
      <div class="logo">搜索中间件</div>
      <el-menu :default-active="$route.path" router background-color="#001529" text-color="#c8c9cc"
        active-text-color="#fff">
        <el-menu-item index="/dashboard"><el-icon><Odometer /></el-icon>仪表盘</el-menu-item>
        <el-menu-item index="/search-test"><el-icon><Search /></el-icon>搜索测试</el-menu-item>
        <el-menu-item index="/sync"><el-icon><Refresh /></el-icon>同步中心</el-menu-item>
        <el-menu-item index="/schedules"><el-icon><Timer /></el-icon>定时任务</el-menu-item>
        <el-menu-item index="/logs"><el-icon><Document /></el-icon>日志中心</el-menu-item>
        <el-menu-item index="/reconcile"><el-icon><Checked /></el-icon>对账中心</el-menu-item>
        <el-menu-item index="/indexes"><el-icon><Setting /></el-icon>索引配置</el-menu-item>
        <el-menu-item index="/synonyms"><el-icon><Link /></el-icon>同义词</el-menu-item>
        <el-menu-item index="/alerts"><el-icon><Bell /></el-icon>告警中心</el-menu-item>
        <el-menu-item v-if="auth.isAdmin" index="/users"><el-icon><User /></el-icon>用户管理</el-menu-item>
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
            <span class="user">{{ auth.user?.username }}（{{ auth.user?.role }}）</span>
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
.aside { background: #001529; }
.logo { color: #fff; font-size: 18px; font-weight: 600; text-align: center; padding: 18px 0; }
.header { display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--el-border-color-light); }
.header-right { display: flex; align-items: center; gap: 12px; }
.page-title { font-size: 16px; font-weight: 600; }
.user { cursor: pointer; color: var(--el-color-primary); }
.main { background: var(--el-bg-color-page); }
html.dark .header { border-bottom-color: #2c2c2d; }
</style>
