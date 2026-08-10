<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between">
        <span>定时任务</span>
        <el-button type="primary" size="small" @click="showDialog = true">新建任务</el-button>
      </div>
    </template>
    <el-table stripe :data="schedules" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="index_name" label="索引" />
      <el-table-column prop="type" label="类型" width="100" />
      <el-table-column prop="cron_expr" label="Cron 表达式" />
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="(v) => toggle(row, v)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="showDialog" title="新建定时任务" width="480px">
    <el-form label-width="100px">
      <el-form-item label="索引">
        <el-select v-model="form.index_name" style="width: 100%">
          <el-option v-for="i in indexes" :key="i" :label="i" :value="i" />
        </el-select>
      </el-form-item>
      <el-form-item label="类型">
        <el-radio-group v-model="form.type">
          <el-radio value="incremental">增量</el-radio>
          <el-radio value="full">全量</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="Cron">
        <el-input v-model="form.cron_expr" placeholder="*/5 * * * * *" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showDialog = false">取消</el-button>
      <el-button type="primary" @click="create">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const schedules = ref([])
const indexes = ref([])
const loading = ref(false)
const showDialog = ref(false)
const form = reactive({ index_name: '', type: 'incremental', cron_expr: '*/5 * * * * *' })

async function load() {
  loading.value = true
  try {
    schedules.value = (await api.listSchedules().catch(() => [])) || []
    indexes.value = (await api.listIndexes().catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!form.index_name) return ElMessage.warning('请选择索引')
  try {
    await api.createSchedule({ ...form })
    ElMessage.success('已创建')
    showDialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function toggle(row, v) {
  try {
    await api.toggleSchedule(row.id, v)
    ElMessage.success(v ? '已启用' : '已停用')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function remove(row) {
  await ElMessageBox.confirm('确认删除该定时任务？', '提示', { type: 'warning' })
  try {
    await api.deleteSchedule(row.id)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
