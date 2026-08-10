<template>
  <div>
    <el-alert v-if="healthError" :title="healthError" type="warning" show-icon :closable="false" style="margin-bottom: 16px" />

    <!-- 指标统计卡 -->
    <el-row :gutter="16" style="margin-bottom: 16px">
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-icon" style="background: #e9effe; color: #3b6cf6">📚</div>
          <div>
            <div class="stat-value">{{ indexes.length }}</div>
            <div class="stat-label">索引</div>
          </div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-icon" style="background: #e8f7ef; color: #34a853">🔄</div>
          <div>
            <div class="stat-value">{{ totalRuns }}</div>
            <div class="stat-label">同步次数</div>
          </div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-icon" style="background: #fdf3e3; color: #f5a623">⏱️</div>
          <div>
            <div class="stat-value">{{ runningCount }}</div>
            <div class="stat-label">运行中</div>
          </div>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="stat-card">
          <div class="stat-icon" style="background: #fdeaea; color: #e5484d">⚠️</div>
          <div>
            <div class="stat-value">{{ failedCount }}</div>
            <div class="stat-label">失败</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 索引卡片 -->
    <el-row :gutter="16" style="margin-bottom: 16px">
      <el-col v-for="idx in indexes" :key="idx" :span="6" style="margin-bottom: 16px">
        <el-card shadow="hover" class="idx-card">
          <div class="idx-head">
            <span class="idx-name">{{ idx }}</span>
            <el-tag :type="running[idx] ? 'warning' : 'success'" size="small" effect="light" round>
              {{ running[idx] ? '同步中' : '正常' }}
            </el-tag>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 4px">
      <template #header>运行状态</template>
      <el-table :data="runs" size="small" v-loading="loading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="index_name" label="索引" />
        <el-table-column prop="type" label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="row.type === 'full' ? 'primary' : 'info'">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small" effect="light" round>{{ row.status }}</el-tag>
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
import { ref, computed, onMounted } from 'vue'
import { api } from '../api'

const indexes = ref([])
const runs = ref([])
const running = ref({})
const loading = ref(false)
const healthError = ref('')

const totalRuns = computed(() => runs.value.length)
const runningCount = computed(() => runs.value.filter((r) => r.status === 'running' || r.status === 'pending').length)
const failedCount = computed(() => runs.value.filter((r) => r.status === 'failed' || r.status === 'partial').length)

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
    const runningSet = {}
    for (const r of runList || []) {
      if (r.status === 'running' || r.status === 'pending') runningSet[r.index_name] = true
    }
    running.value = runningSet
  } finally {
    loading.value = false
  }
  api.health().catch((e) => { healthError.value = '后端健康检查异常：' + e.message })
}

onMounted(load)
</script>

<style scoped>
.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px;
  padding: 22px 20px;
  transition: box-shadow var(--app-transition), transform var(--app-transition);
}
.stat-card:hover { box-shadow: 0 6px 18px rgba(0, 0, 0, 0.1); transform: translateY(-2px); }
.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
}
.stat-value {
  font-size: 30px;
  font-weight: 700;
  line-height: 1.1;
  font-family: var(--app-font-mono);
}
.stat-label { font-size: 14px; color: var(--el-text-color-secondary); margin-top: 4px; }

.idx-card { margin-bottom: 0; }
.idx-head { display: flex; align-items: center; justify-content: space-between; }
.idx-name { font-size: 15px; font-weight: 600; }
</style>
