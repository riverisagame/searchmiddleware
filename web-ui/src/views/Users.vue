<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>用户管理（admin）</span>
        <el-button type="primary" size="small" @click="showDialog = true">新建用户</el-button>
      </div>
    </template>
    <el-table :data="users" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="role" label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">{{ row.role }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" />
    </el-table>
  </el-card>

  <el-dialog v-model="showDialog" title="新建用户" width="420px">
    <el-form label-width="80px">
      <el-form-item label="用户名">
        <el-input v-model="form.username" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input v-model="form.password" type="password" show-password />
      </el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="form.role">
          <el-radio value="viewer">viewer</el-radio>
          <el-radio value="admin">admin</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showDialog = false">取消</el-button>
      <el-button type="primary" @click="create">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const users = ref([])
const loading = ref(false)
const showDialog = ref(false)
const form = reactive({ username: '', password: '', role: 'viewer' })

async function load() {
  loading.value = true
  try {
    users.value = (await api.listUsers().catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!form.username || !form.password) return ElMessage.warning('请输入用户名和密码')
  try {
    await api.createUser({ ...form })
    ElMessage.success('已创建')
    showDialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
