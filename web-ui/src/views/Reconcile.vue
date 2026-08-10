<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>对账中心</span>
        <el-select v-model="indexName" placeholder="选择索引" style="width: 200px">
          <el-option v-for="i in indexes" :key="i" :label="i" :value="i" />
        </el-select>
      </div>
    </template>
    <el-table stripe :data="results" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="index_name" label="索引" />
      <el-table-column prop="type" label="级别" width="90">
        <template #default="{ row }">
          <el-tag :type="row.type === 'count' ? 'info' : 'warning'" size="small">{{ row.type === 'count' ? 'COUNT' : 'ID级' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="index_count" label="索引数" width="90" />
      <el-table-column prop="db_valid_count" label="库有效数" width="90" />
      <el-table-column label="缺失" width="80">
        <template #default="{ row }">{{ parseIds(row.missing_ids).length }}</template>
      </el-table-column>
      <el-table-column label="脏文档" width="80">
        <template #default="{ row }">{{ parseIds(row.extra_ids).length }}</template>
      </el-table-column>
      <el-table-column prop="created_at" label="时间" width="180" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button v-if="row.type === 'id' && (parseIds(row.missing_ids).length || parseIds(row.extra_ids).length)"
            type="primary" size="small" @click="fix(row)">一键补同步</el-button>
          <span v-else style="color: #909399">—</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const indexes = ref([])
const indexName = ref('')
const results = ref([])
const loading = ref(false)

function parseIds(s) {
  try { return JSON.parse(s || '[]') } catch { return [] }
}

async function load() {
  if (!indexName.value) { results.value = []; return }
  loading.value = true
  try {
    results.value = (await api.listReconcile(indexName.value).catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

async function fix(row) {
  await ElMessageBox.confirm('将重建缺失文档并删除脏文档，确认执行？', '一键补同步', { type: 'warning' })
  try {
    await api.fixReconcile(row.index_name, row.id)
    ElMessage.success('补同步完成')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

watch(indexName, load)
onMounted(async () => {
  indexes.value = (await api.listIndexes().catch(() => [])) || []
})
</script>
