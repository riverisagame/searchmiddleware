<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>同义词管理（查询期扩展，保存即生效）</span>
        <div>
          <el-button size="small" @click="syncToZinc">同步到 Zinc</el-button>
          <el-button type="primary" size="small" @click="showDialog = true">新增</el-button>
        </div>
      </div>
    </template>
    <el-table stripe :data="synonyms" v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="word" label="词" width="150" />
      <el-table-column label="同义词">
        <template #default="{ row }">{{ row.synonyms }}</template>
      </el-table-column>
      <el-table-column prop="indexes" label="作用索引" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="showDialog" title="新增同义词" width="460px">
    <el-form label-width="80px">
      <el-form-item label="词">
        <el-input v-model="form.word" placeholder="如：手机" />
      </el-form-item>
      <el-form-item label="同义词">
        <el-input v-model="form.synonyms" placeholder='JSON 数组，如 ["移动电话","handset"]' />
      </el-form-item>
      <el-form-item label="作用索引">
        <el-input v-model="form.indexes" placeholder="maintenance 或留空" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="showDialog = false">取消</el-button>
      <el-button type="primary" @click="create">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const synonyms = ref([])
const loading = ref(false)
const showDialog = ref(false)
const form = reactive({ word: '', synonyms: '', indexes: '' })

async function load() {
  loading.value = true
  try {
    synonyms.value = (await api.listSynonyms().catch(() => [])) || []
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!form.word) return ElMessage.warning('请输入词')
  try {
    await api.createSynonym({ ...form })
    ElMessage.success('已保存，查询期即时生效')
    showDialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function remove(row) {
  await ElMessageBox.confirm('确认删除该同义词？', '提示', { type: 'warning' })
  try {
    await api.deleteSynonym(row.id)
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function syncToZinc() {
  try {
    await api.syncSynonymsToZinc()
    ElMessage.success('已同步到 Zinc 并触发重载')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>
