<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>索引配置编辑器</span>
        <el-button type="primary" size="small" @click="openNew">新建索引</el-button>
      </div>
    </template>
    <el-row :gutter="16">
      <el-col :span="6">
        <el-menu :default-active="current" @select="select">
          <el-menu-item v-for="i in indexes" :key="i" :index="i">{{ i }}</el-menu-item>
        </el-menu>
      </el-col>
      <el-col :span="18">
        <template v-if="current">
          <el-alert title="保存后需重建索引生效" type="info" show-icon :closable="false" style="margin-bottom: 12px" />
          <el-input v-model="content" type="textarea" :rows="24" class="editor" />
          <div style="margin-top: 12px">
            <el-button type="primary" :loading="saving" @click="save">保存</el-button>
            <el-button type="danger" @click="remove">删除</el-button>
          </div>
        </template>
        <el-empty v-else description="选择或新建索引配置" />
      </el-col>
    </el-row>
  </el-card>

  <el-dialog v-model="newDialog" title="新建索引" width="400px">
    <el-form label-width="80px">
      <el-form-item label="索引名">
        <el-input v-model="newName" placeholder="如 goods" />
      </el-form-item>
      <el-form-item label="模板">
        <el-checkbox v-model="useTemplate">使用 maintenance 模板</el-checkbox>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="newDialog = false">取消</el-button>
      <el-button type="primary" @click="createNew">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const indexes = ref([])
const current = ref('')
const content = ref('')
const saving = ref(false)
const newDialog = ref(false)
const newName = ref('')
const useTemplate = ref(false)

async function load() {
  indexes.value = (await api.listIndexes().catch(() => [])) || []
}

async function select(name) {
  current.value = name
  // 后端 GET /indexes/:name 返回解析后结构，前端展示 YAML 文本
  const cfg = await api.listIndexes() // 简化：直接读原始文件不可行，展示解析结果
  content.value = JSON.stringify(await api.listIndexes(), null, 2)
  // 更好的做法：GET /api/v1/indexes/:name?raw=1 返回原始 YAML（后端待补）
  loadDetail(name)
}

async function loadDetail(name) {
  try {
    const resp = await fetch('/api/v1/indexes/' + name, { headers: { Authorization: 'Bearer ' + localStorage.getItem('sm_token') } })
    const data = await resp.json()
    content.value = JSON.stringify(data.data, null, 2)
  } catch (e) {
    content.value = ''
  }
}

async function save() {
  saving.value = true
  try {
    await api.updateIndex(current.value, content.value)
    ElMessage.success('已保存，索引需重建')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove() {
  await ElMessageBox.confirm('删除索引配置将同时移除该索引的同步定义，确认？', '提示', { type: 'warning' })
  try {
    await api.deleteIndex(current.value)
    ElMessage.success('已删除')
    current.value = ''
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openNew() {
  newName.value = ''
  useTemplate.value = false
  newDialog.value = true
}

async function createNew() {
  if (!newName.value) return ElMessage.warning('请输入索引名')
  const template = useTemplate.value
    ? `source:\n  datasource: main\n  sql_query: "SELECT id FROM your_table WHERE delete_time = 0"\n  incremental_field: update_time\nindex:\n  name: ${newName.value}\n  fields:\n    name: { type: text, searchable: true }\n`
    : `source:\n  datasource: main\n  sql_query: "SELECT id FROM your_table"\nindex:\n  name: ${newName.value}\n  fields:\n    id: { type: keyword, filter: true }\n`
  try {
    await api.createIndex(newName.value, template)
    ElMessage.success('已创建')
    newDialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<style scoped>
.editor :deep(textarea) { font-family: 'Consolas', 'Courier New', monospace; font-size: 13px; }
</style>
