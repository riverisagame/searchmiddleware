<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>日志中心</span>
        <div>
          <el-select v-model="indexFilter" placeholder="索引" clearable style="width: 160px; margin-right: 8px">
            <el-option v-for="i in indexes" :key="i" :label="i" :value="i" />
          </el-select>
          <el-select v-model="levelFilter" placeholder="级别" clearable style="width: 110px">
            <el-option label="INFO" value="INFO" />
            <el-option label="WARN" value="WARN" />
            <el-option label="ERROR" value="ERROR" />
          </el-select>
        </div>
      </div>
    </template>
    <el-table stripe :data="logs" v-loading="loading" size="small">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="index_name" label="索引" />
      <el-table-column prop="level" label="级别" width="80">
        <template #default="{ row }">
          <el-tag :type="row.level === 'ERROR' ? 'danger' : row.level === 'WARN' ? 'warning' : 'info'" size="small">{{ row.level }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="task" label="任务" width="100" />
      <el-table-column prop="message" label="消息" show-overflow-tooltip />
      <el-table-column prop="created_at" label="时间" width="180" />
    </el-table>
  </el-card>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { api } from '../api'

const logs = ref([])
const indexes = ref([])
const indexFilter = ref('')
const levelFilter = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    logs.value = (await api.listLogs({ index: indexFilter.value, level: levelFilter.value }).catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

watch([indexFilter, levelFilter], load)
onMounted(async () => {
  indexes.value = (await api.listIndexes().catch(() => [])) || []
  load()
})
</script>
