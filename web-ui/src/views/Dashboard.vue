<template>
  <div>
    <el-alert v-if="healthError" :title="healthError" type="warning" show-icon :closable="false" style="margin-bottom: 16px" />

    <el-row :gutter="16">
      <el-col v-for="idx in indexes" :key="idx" :span="6">
        <el-card shadow="hover" class="idx-card">
          <div class="idx-name">{{ idx }}</div>
          <div class="idx-status">
            <el-tag :type="running[idx] ? 'warning' : 'success'" size="small">
              {{ running[idx] ? '同步中' : '正常' }}
            </el-tag>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 16px">
      <template #header>运行状态</template>
      <el-table :data="runs" size="small" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="index_name" label="索引" />
        <el-table-column prop="type" label="类型" width="90" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rows_count" label="行数" width="80" />
        <el-table-column label="耗时(ms)" width="90">
          <template #default="{ row }">{{ row.duration_ms }}</template>
        </el-table-column>
        <el-table-column prop="started_at" label="开始时间" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const indexes = ref([])
const runs = ref([])
const running = ref({})
const loading = ref(false)
const healthError = ref('')

function statusType(s) {
  return { success: 'success', failed: 'danger', partial: 'warning', skipped: 'info', interrupted: 'warning' }[s] || 'info'
}

async function load() {
  loading.value = true
  try {
    const [idxList, runList] = await Promise.all([
      api.listIndexes().catch(() => []),
      api.listRuns().catch(() => []),
    ])
    indexes.value = idxList || []
    runs.value = (runList || []).slice(0, 20)
  } finally {
    loading.value = false
  }
  api.health().catch((e) => { healthError.value = '后端健康检查异常：' + e.message })
}

onMounted(load)
</script>

<style scoped>
.idx-card { margin-bottom: 16px; }
.idx-name { font-size: 16px; font-weight: 600; }
.idx-status { margin-top: 8px; }
</style>
