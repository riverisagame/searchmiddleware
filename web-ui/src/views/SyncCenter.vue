<template>
  <el-card>
    <template #header>手动同步</template>
    <el-form inline>
      <el-form-item label="索引">
        <el-select v-model="indexName" placeholder="选择索引" style="width: 200px">
          <el-option v-for="i in indexes" :key="i" :label="i" :value="i" />
        </el-select>
      </el-form-item>
      <el-form-item label="类型">
        <el-select v-model="syncType" style="width: 140px">
          <el-option label="全量重建" value="full" />
          <el-option label="增量同步" value="incremental" />
          <el-option label="指定 ID" value="by_ids" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="syncType === 'by_ids'" label="IDs（逗号分隔）">
        <el-input v-model="idsStr" placeholder="11073,11074" style="width: 200px" />
      </el-form-item>
      <el-button type="primary" :loading="syncing" @click="doSync">执行</el-button>
      <el-button @click="doReconcile('count')">对账(COUNT)</el-button>
      <el-button @click="doReconcile('id')">对账(ID级)</el-button>
    </el-form>
  </el-card>

  <el-card style="margin-top: 16px">
    <template #header>运行历史</template>
    <el-table stripe :data="runs" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="index_name" label="索引" />
      <el-table-column prop="type" label="类型" width="90" />
      <el-table-column prop="trigger" label="触发" width="80" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small" round>{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="rows_count" label="行数" width="80" />
      <el-table-column label="耗时(ms)" width="90">
        <template #default="{ row }">{{ row.duration_ms }}</template>
      </el-table-column>
      <el-table-column label="吞吐" width="90">
        <template #default="{ row }">{{ row.throughput?.toFixed?.(1) ?? row.throughput }}</template>
      </el-table-column>
      <el-table-column prop="started_at" label="开始时间" />
    </el-table>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const indexes = ref([])
const runs = ref([])
const indexName = ref('')
const syncType = ref('incremental')
const idsStr = ref('')
const syncing = ref(false)
const loading = ref(false)

function statusType(s) {
  return { success: 'success', failed: 'danger', partial: 'warning', skipped: 'info', interrupted: 'warning' }[s] || 'info'
}

async function load() {
  loading.value = true
  try {
    indexes.value = (await api.listIndexes().catch(() => [])) || []
    runs.value = (await api.listRuns().catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

async function doSync() {
  if (!indexName.value) return ElMessage.warning('请选择索引')
  syncing.value = true
  try {
    let ids
    if (syncType.value === 'by_ids') {
      ids = idsStr.value.split(',').map((s) => s.trim()).filter(Boolean)
      if (!ids.length) return ElMessage.warning('请输入 IDs')
    }
    await api.syncIndex(indexName.value, syncType.value, ids)
    ElMessage.success('同步已触发')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    syncing.value = false
  }
}

async function doReconcile(type) {
  if (!indexName.value) return ElMessage.warning('请选择索引')
  try {
    const r = await api.reconcile(indexName.value, type)
    ElMessage.success(`对账完成：索引 ${r.index_count} / 库 ${r.db_valid_count}，缺 ${JSON.parse(r.missing_ids || '[]').length} 条，脏 ${JSON.parse(r.extra_ids || '[]').length} 条`)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
