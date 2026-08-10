<template>
  <div class="login-wrap">
    <!-- 装饰背景 -->
    <div class="bg-glow glow-1"></div>
    <div class="bg-glow glow-2"></div>
    <div class="bg-grid"></div>

    <el-card class="login-card" shadow="always">
      <div class="brand">
        <div class="brand-icon">🔍</div>
        <h2 class="title">搜索中间件管理台</h2>
        <p class="subtitle">Search Middleware Console</p>
      </div>
      <el-form @submit.prevent="onSubmit">
        <el-form-item>
          <el-input v-model="username" placeholder="用户名" size="large" :prefix-icon="User" clearable />
        </el-form-item>
        <el-form-item>
          <el-input v-model="password" type="password" placeholder="密码" size="large" :prefix-icon="Lock" show-password @keyup.enter="onSubmit" />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="onSubmit">
          {{ loading ? '登录中...' : '登 录' }}
        </el-button>
      </el-form>
      <p class="hint">首次使用：命令行执行 <code>searchmiddleware user:create admin 密码 admin</code></p>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { User, Lock } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const loading = ref(false)

async function onSubmit() {
  if (!username.value || !password.value) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    ElMessage.success('登录成功')
    router.push('/')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  background: #f5f7fa;
}

/* 科技感装饰：网格 + 光晕 */
.bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(13, 148, 136, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(13, 148, 136, 0.05) 1px, transparent 1px);
  background-size: 40px 40px;
  mask-image: radial-gradient(ellipse 60% 60% at 50% 40%, black, transparent);
}
.bg-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.35;
}
.glow-1 { width: 420px; height: 420px; background: #0d9488; top: -120px; left: -80px; }
.glow-2 { width: 380px; height: 380px; background: #22d3ee; bottom: -100px; right: -60px; }

.login-card {
  width: 400px;
  border-radius: 18px;
  position: relative;
  z-index: 1;
  padding: 30px 26px;
  backdrop-filter: blur(8px);
}
.brand { text-align: center; margin-bottom: 26px; }
.brand-icon { font-size: 40px; margin-bottom: 10px; }
.title { margin: 0 0 4px; font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); }
.subtitle { margin: 0; font-size: 12px; letter-spacing: 1px; color: var(--el-text-color-secondary); font-family: var(--app-font-mono); }
.hint { color: var(--el-text-color-secondary); font-size: 12px; text-align: center; margin-top: 18px; }
.hint code { background: var(--el-fill-color-lighter); padding: 1px 6px; border-radius: 4px; font-size: 11px; }

/* 暗色：科技感强化 */
html.dark .login-wrap { background: var(--el-bg-color-page); }
html.dark .glow-1 { background: rgba(34, 211, 238, 0.5); }
html.dark .glow-2 { background: rgba(34, 197, 94, 0.35); }
html.dark .bg-grid {
  background-image:
    linear-gradient(rgba(34, 211, 238, 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(34, 211, 238, 0.06) 1px, transparent 1px);
}
html.dark .login-card {
  background: rgba(23, 26, 31, 0.85);
  border: 1px solid rgba(34, 211, 238, 0.15);
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.6), 0 0 24px rgba(34, 211, 238, 0.08);
}
</style>
