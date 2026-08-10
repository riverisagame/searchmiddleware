<template>
  <el-card>
    <template #header>告警中心</template>
    <el-table stripe :data="alerts" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="index_name" label="索引" />
      <el-table-column prop="level" label="级别" width="90">
        <template #default="{ row }">
          <el-tag :type="row.level === 'ERROR' ? 'danger' : row.level === 'WARN' ? 'warning' : 'info'" size="small" round>{{ row.level }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="消息" show-overflow-tooltip />
      <el-table-column prop="created_at" label="时间" width="180" />
    </el-table>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const alerts = ref([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    alerts.value = (await api.listAlerts().catch(() => [])) || []
  } finally {
    loading.value = false
  }
})
</script>
