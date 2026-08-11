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

    <!-- 运行状态分析（产品级可视化，无依赖 SVG） -->
    <el-row :gutter="16" style="margin-bottom: 16px">
      <el-col :span="12">
        <el-card class="viz-card">
          <template #header>同步状态分布（近 {{ runs.length }} 次）</template>
          <div v-if="runs.length" class="viz-body">
            <div class="stack-bar">
              <div v-for="seg in statusSegments" :key="seg.key" :class="['stack-seg', seg.key]"
                :style="{ width: seg.pct + '%' }" :title="seg.label + '：' + seg.count + ' 次'"></div>
            </div>
            <div class="stack-legend">
              <span v-for="seg in statusSegments" :key="seg.key" class="legend-item">
                <i :class="['dot', seg.key]"></i>{{ seg.label }} <b class="num">{{ seg.count }}</b>
              </span>
            </div>
          </div>
          <el-empty v-else description="暂无运行记录" :image-size="60" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card class="viz-card">
          <template #header>最近运行趋势</template>
          <div v-if="runs.length" class="viz-body">
            <div class="trend-row">
              <div v-for="(r, i) in recentTrend" :key="i" class="trend-dot-wrap" :title="`${r.index_name} ${r.status} (${r.duration_ms}ms)`">
                <div :class="['trend-dot', 'dot-' + r.status]"></div>
                <div class="trend-idx">{{ r.index_name.slice(0, 4) }}</div>
              </div>
            </div>
            <div class="trend-hint">近 {{ recentTrend.length }} 次运行（左新右旧）· 绿=成功 红=失败 黄=部分/中断 灰=跳过</div>
          </div>
          <el-empty v-else description="暂无运行记录" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 4px">
      <template #header>运行状态</template>
      <el-table stripe :data="runs" size="small" v-loading="loading">
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
        <el-table-column prop="rows_count" label="行数" width="80" class-name="num" />
        <el-table-column label="耗时(ms)" width="90" class-name="num">
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

// 状态分布（堆叠条 + 图例，值常显=AAA 无障碍）
const statusSegments = computed(() => {
  const defs = [
    { key: 'success', label: '成功', color: '#34a853' },
    { key: 'failed', label: '失败', color: '#e5484d' },
    { key: 'partial', label: '部分', color: '#f5a623' },
    { key: 'skipped', label: '跳过', color: '#909399' },
  ]
  const counts = { success: 0, failed: 0, partial: 0, skipped: 0, interrupted: 0 }
  for (const r of runs.value) {
    const k = r.status === 'interrupted' ? 'partial' : r.status
    if (counts[k] !== undefined) counts[k]++
  }
  const total = runs.value.length || 1
  return defs.map((d) => ({
    ...d,
    count: counts[d.key] || 0,
    pct: Math.round(((counts[d.key] || 0) / total) * 100),
  })).filter((d) => d.count > 0)
})

// 最近趋势（左新右旧）
const recentTrend = computed(() => {
  const order = { success: 0, partial: 1, interrupted: 1, skipped: 2, failed: 3, running: 4, pending: 4 }
  return [...runs.value].sort((a, b) => new Date(b.started_at) - new Date(a.started_at)).slice(0, 12)
})

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

/* 可视化 */
.viz-card { height: 100%; }
.viz-body { padding: 8px 4px; }
.stack-bar {
  display: flex;
  height: 22px;
  border-radius: 6px;
  overflow: hidden;
  background: var(--el-fill-color-lighter);
}
.stack-seg { height: 100%; transition: width 0.4s ease; }
.stack-seg.success { background: #34a853; }
.stack-seg.failed { background: #e5484d; }
.stack-seg.partial { background: #f5a623; }
.stack-seg.skipped { background: #909399; }
.stack-legend { display: flex; gap: 18px; margin-top: 12px; flex-wrap: wrap; }
.legend-item { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--el-text-color-regular); }
.legend-item b { font-weight: 600; color: var(--el-text-color-primary); }
.dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.dot.success, .trend-dot.dot-success { background: #34a853; }
.dot.failed, .trend-dot.dot-failed { background: #e5484d; }
.dot.partial, .trend-dot.dot-partial, .trend-dot.dot-interrupted { background: #f5a623; }
.dot.skipped, .trend-dot.dot-skipped { background: #909399; }
.trend-row { display: flex; gap: 10px; overflow-x: auto; padding-bottom: 6px; }
.trend-dot-wrap { display: flex; flex-direction: column; align-items: center; gap: 4px; min-width: 34px; }
.trend-dot { width: 14px; height: 14px; border-radius: 50%; box-shadow: 0 0 6px currentColor; }
.trend-dot.dot-running, .trend-dot.dot-pending { background: #3b6cf6; box-shadow: 0 0 8px rgba(59, 108, 246, 0.6); }
.trend-idx { font-size: 10px; color: var(--el-text-color-secondary); font-family: var(--app-font-mono); }
.trend-hint { margin-top: 10px; font-size: 12px; color: var(--el-text-color-secondary); }
html.dark .trend-dot { box-shadow: 0 0 8px currentColor; }
</style>
