<template>
  <el-card>
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center">
        <span>索引配置编辑器</span>
        <div>
          <el-button size="small" @click="switchMode">{{ mode === 'form' ? '高级模式(JSON)' : '表单模式' }}</el-button>
          <el-button type="primary" size="small" @click="openNew">新建索引</el-button>
        </div>
      </div>
    </template>
    <el-row :gutter="16">
      <el-col :span="5">
        <el-menu :default-active="current" @select="select">
          <el-menu-item v-for="i in indexes" :key="i" :index="i">{{ i }}</el-menu-item>
        </el-menu>
      </el-col>
      <el-col :span="19">
        <template v-if="current">
          <el-alert title="保存后需重建索引生效" type="info" show-icon :closable="false" style="margin-bottom: 12px" />

          <!-- ===== 表单模式：Tab 布局（基本信息 / 属性 / 字段）===== -->
          <template v-if="mode === 'form'">
            <el-tabs v-model="activeTab">
              <!-- Tab 1：基本信息 + 主 SQL -->
              <el-tab-pane label="基本信息" name="basic">
                <el-form label-width="100px" size="default">
                  <el-row :gutter="16">
                    <el-col :span="8">
                      <el-form-item label="索引名">
                        <el-input v-model="form.index.name" :disabled="true" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="8">
                      <el-form-item label="数据源">
                        <el-input v-model="form.source.datasource" placeholder="datasources.yaml 中的名称" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="8">
                      <el-form-item label="增量字段">
                        <el-input v-model="form.source.incremental_field" placeholder="如 update_time" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="8">
                      <el-form-item label="分析器">
                        <el-input v-model="form.index.analyzer" placeholder="jieba_std" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="8">
                      <el-form-item label="搜索分析器">
                        <el-input v-model="form.index.search_analyzer" placeholder="jieba_search" />
                      </el-form-item>
                    </el-col>
                  </el-row>

                  <div class="section-title">主 SQL</div>
                  <el-form-item label="SQL 查询">
                    <el-input v-model="form.source.sql_query" type="textarea" :rows="18" placeholder="SELECT id, name FROM your_table WHERE delete_time = 0" class="mono sql-editor" />
                  </el-form-item>
                </el-form>
              </el-tab-pane>

              <!-- Tab 2：属性（属性 SQL + 权重） -->
              <el-tab-pane label="属性" name="attrs">
                <div class="section-title">属性 SQL（GROUP_CONCAT 合并）</div>
                <div v-for="(jf, i) in form.source.joined_fields" :key="jf._uid" style="margin-bottom: 16px">
                  <div style="display: flex; gap: 8px; margin-bottom: 6px">
                    <el-input v-model="jf.key" placeholder="字段名，如 category_names" style="width: 220px" />
                    <el-button type="danger" plain @click="removeJoined(i)">删除</el-button>
                  </div>
                  <el-input v-model="jf.value" type="textarea" :rows="6" placeholder="SELECT r.id, GROUP_CONCAT(...) AS val FROM ..." class="mono sql-editor" />
                </div>
                <el-button type="primary" plain @click="addJoined">+ 添加属性 SQL</el-button>

                <div class="section-title" style="margin-top: 24px">权重（boost）</div>
                <div v-for="(b, i) in form.index.boost" :key="b._uid" style="display: flex; gap: 8px; margin-bottom: 10px; max-width: 480px">
                  <el-input v-model="b.key" placeholder="字段名" />
                  <el-input v-model="b.value" placeholder="权重" style="width: 140px" />
                  <el-button type="danger" plain @click="removeBoost(i)">删除</el-button>
                </div>
                <el-button type="primary" plain @click="addBoost">+ 添加权重</el-button>
              </el-tab-pane>

              <!-- Tab 3：字段信息 -->
              <el-tab-pane label="字段" name="fields">
                <el-alert type="info" :closable="false" style="margin-bottom: 12px"
                  title="字段 = 主 SQL 返回的列；类型在此配置（text 分词搜索 / keyword 精确 / numeric·float 数值 / date 日期）。数组 = 多值字段（如 category_ids=[1,2,3]），勾选后需指定元素类型。" />
                <div class="section-title" style="display: flex; justify-content: space-between; align-items: center">
                  <span>字段列表</span>
                  <el-button type="primary" plain size="small" @click="addField">+ 添加字段</el-button>
                </div>
                <el-table stripe :data="form.index.fields" size="default" border height="520" style="width: 100%">
                  <el-table-column label="字段名" width="380" show-overflow-tooltip>
                    <template #default="{ row }"><el-input v-model="row.name" size="default" /></template>
                  </el-table-column>
                  <el-table-column label="类型" width="180">
                    <template #default="{ row }">
                      <el-select v-model="row.type" size="default" style="width: 100%" @change="(v) => onTypeChange(row, v)">
                        <el-option v-for="t in ['text','keyword','numeric','float','date']" :key="t" :label="t" :value="t" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="搜索" width="100" align="center">
                    <template #default="{ row }"><el-checkbox v-model="row.searchable" /></template>
                  </el-table-column>
                  <el-table-column label="过滤" width="100" align="center">
                    <template #default="{ row }"><el-checkbox v-model="row.filter" /></template>
                  </el-table-column>
                  <el-table-column label="排序" width="100" align="center">
                    <template #default="{ row }"><el-checkbox v-model="row.sortable" /></template>
                  </el-table-column>
                  <el-table-column label="聚合" width="100" align="center">
                    <template #default="{ row }"><el-checkbox v-model="row.agg" /></template>
                  </el-table-column>
                  <el-table-column label="数组" width="100" align="center">
                    <template #header>
                      <el-tooltip content="多值字段（如 category_ids=[1,2,3]）勾选后指定元素类型" placement="top">
                        <span>数组 <el-icon style="vertical-align: -2px"><QuestionFilled /></el-icon></span>
                      </el-tooltip>
                    </template>
                    <template #default="{ row }">
                      <el-checkbox v-model="row.is_array" @change="(v) => onArrayToggle(row, v)" />
                    </template>
                  </el-table-column>
                  <el-table-column label="元素类型" width="170">
                    <template #default="{ row }">
                      <el-select v-model="row.element_type" size="default" clearable placeholder="数组元素" style="width: 100%" :disabled="!row.is_array">
                        <el-option v-for="t in ['long','integer','short','byte','double','float','text','keyword','boolean']" :key="t" :label="t" :value="t" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="格式" width="220">
                    <template #default="{ row }">
                      <el-select v-model="row.format" size="default" clearable placeholder="仅 date 类型" style="width: 100%" :disabled="row.type !== 'date'">
                        <el-option v-for="f in ['unix_timestamp','unix_milli','date_time','date','strict_date_optional_time','yyyy-MM-dd HH:mm:ss','2006-01-02 15:04:05']" :key="f" :label="f" :value="f" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column width="80" align="center">
                    <template #default="{ $index }">
                      <el-button type="danger" plain size="small" @click="removeField($index)">删</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>
            </el-tabs>
          </template>

          <!-- ===== 高级模式 ===== -->
          <el-input v-else v-model="content" type="textarea" :rows="24" class="editor" />

          <div style="margin-top: 12px">
            <el-button type="primary" :loading="saving" @click="save">保存</el-button>
            <el-button type="danger" @click="remove">删除</el-button>
            <el-button @click="openSqlTest">试跑 SQL</el-button>
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

  <el-dialog v-model="sqlDialog" title="试跑 SQL（仅 SELECT，自动 LIMIT 20）" width="900px">
    <el-form label-width="80px">
      <el-form-item label="数据源">
        <el-input v-model="sqlDS" placeholder="datasources.yaml 中的名称，如 main" style="width: 220px" />
      </el-form-item>
      <el-form-item label="SQL">
        <el-input v-model="sqlText" type="textarea" :rows="10" placeholder="SELECT id, name FROM your_table WHERE ..." class="mono sql-editor" />
      </el-form-item>
    </el-form>
    <el-button type="primary" :loading="sqlLoading" @click="runSqlTest" style="margin-left: 80px">执行</el-button>
    <el-table stripe v-if="sqlResult" :data="sqlResult.rows" size="small" border style="margin-top: 12px" max-height="300">
      <el-table-column v-for="col in sqlResult.columns" :key="col" :prop="col" :label="col" min-width="100" />
    </el-table>
    <template #footer>
      <el-button @click="sqlDialog = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { QuestionFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../api'

const indexes = ref([])
const current = ref('')
const content = ref('')
const saving = ref(false)
const newDialog = ref(false)
const newName = ref('')
const useTemplate = ref(false)
const mode = ref('form')
const activeTab = ref('basic')
const sqlDialog = ref(false)
const sqlDS = ref('main')
const sqlText = ref('')
const sqlLoading = ref(false)
const sqlResult = ref(null)

const form = reactive({
  source: { datasource: '', sql_query: '', incremental_field: '', joined_fields: [] },
  index: { name: '', analyzer: 'jieba_std', search_analyzer: 'jieba_search', boost: [], fields: [] },
})

let uidSeq = 0

const EMPTY_FIELD = () => ({ name: '', type: 'text', searchable: true, filter: false, sortable: false, agg: false, is_array: false, element_type: '', format: '', analyzer: '', search_analyzer: '' })

// 数组开关联动：勾选数组 → 默认元素类型 long；取消 → 清空元素类型
function onArrayToggle(row, v) {
  if (v && !row.element_type) row.element_type = 'long'
  if (!v) row.element_type = ''
}

// 类型联动：格式仅 date 类型可用（其他类型清空格式）
function onTypeChange(row, v) {
  if (v === 'date') {
    if (!row.format) row.format = 'unix_timestamp'
  } else {
    row.format = ''
  }
}

async function load() {
  indexes.value = (await api.listIndexes().catch(() => [])) || []
}

async function select(name) {
  current.value = name
  content.value = ''
  try {
    const resp = await fetch('/api/v1/indexes/' + name, { headers: { Authorization: 'Bearer ' + localStorage.getItem('sm_token') } })
    const data = await resp.json()
    const cfg = data.data || {}
    content.value = JSON.stringify(cfg, null, 2)
    fillForm(cfg)
  } catch (e) {
    content.value = ''
  }
}

function fillForm(cfg) {
  const src = cfg.Source || {}
  form.source.datasource = src.DataSource || ''
  form.source.sql_query = src.SQLQuery || ''
  form.source.incremental_field = src.IncrementalField || ''
  form.source.joined_fields = Object.entries(src.SQLJoinedField || {}).map(([k, v]) => ({ _uid: ++uidSeq, key: k, value: v }))

  const idx = cfg.Index || {}
  form.index.name = idx.Name || current.value
  form.index.analyzer = idx.Analyzer || 'jieba_std'
  form.index.search_analyzer = idx.SearchAnalyzer || 'jieba_search'
  form.index.boost = Object.entries(idx.Boost || {}).map(([k, v]) => ({ _uid: ++uidSeq, key: k, value: String(v) }))

  const fields = []
  for (const [name, f] of Object.entries(idx.Fields || {})) {
    fields.push({
      name,
      type: f.Type || 'text',
      searchable: !!f.Searchable,
      filter: !!f.Filter,
      sortable: !!f.Sortable,
      agg: !!f.Agg,
      is_array: !!f.ElementType,
      element_type: f.ElementType || '',
      format: f.Format || '',
      analyzer: f.Analyzer || '',
      search_analyzer: f.SearchAnalyzer || '',
    })
  }
  form.index.fields = fields
}

function addField() { form.index.fields.push(EMPTY_FIELD()) }
function removeField(i) { form.index.fields.splice(i, 1) }
function addBoost() { form.index.boost.push({ _uid: ++uidSeq, key: '', value: '1.0' }) }
function removeBoost(i) { form.index.boost.splice(i, 1) }
function addJoined() { form.source.joined_fields.push({ _uid: ++uidSeq, key: '', value: '' }) }
function removeJoined(i) { form.source.joined_fields.splice(i, 1) }

function toYaml() {
  const lines = []
  lines.push('source:')
  lines.push(`  datasource: ${form.source.datasource}`)
  lines.push('  sql_query: |')
  for (const l of String(form.source.sql_query || '').split('\n')) lines.push('    ' + l)
  if (form.source.incremental_field) lines.push(`  incremental_field: ${form.source.incremental_field}`)
  const jf = form.source.joined_fields.filter((x) => x.key)
  if (jf.length) {
    lines.push('  sql_joined_field:')
    for (const j of jf) {
      lines.push(`    ${j.key}: |`)
      for (const l of String(j.value || '').split('\n')) lines.push('      ' + l)
    }
  }
  lines.push('index:')
  lines.push(`  name: ${form.index.name}`)
  lines.push(`  analyzer: ${form.index.analyzer}`)
  lines.push(`  search_analyzer: ${form.index.search_analyzer}`)
  const boosts = form.index.boost.filter((x) => x.key)
  if (boosts.length) {
    lines.push('  boost:')
    for (const b of boosts) lines.push(`    ${b.key}: ${b.value}`)
  }
  lines.push('  fields:')
  for (const f of form.index.fields.filter((x) => x.name)) {
    const opts = []
    if (f.type) opts.push(`type: ${f.type}`)
    if (f.searchable) opts.push('searchable: true')
    if (f.filter) opts.push('filter: true')
    if (f.sortable) opts.push('sortable: true')
    if (f.agg) opts.push('agg: true')
    if (f.element_type) opts.push(`element_type: ${f.element_type}`)
    if (f.format) opts.push(`format: ${f.format}`)
    if (f.analyzer) opts.push(`analyzer: ${f.analyzer}`)
    if (f.search_analyzer) opts.push(`search_analyzer: ${f.search_analyzer}`)
    lines.push(`    ${f.name}: { ${opts.join(', ')} }`)
  }
  return lines.join('\n')
}

function switchMode() {
  if (mode.value === 'form') {
    content.value = toYaml()
    mode.value = 'json'
  } else {
    mode.value = 'form'
  }
}

async function save() {
  saving.value = true
  try {
    const body = mode.value === 'form' ? toYaml() : content.value
    await api.updateIndex(current.value, body)
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
    ? `source:\n  datasource: main\n  sql_query: |\n    SELECT id FROM your_table WHERE delete_time = 0\n  incremental_field: update_time\nindex:\n  name: ${newName.value}\n  fields:\n    name: { type: text, searchable: true }\n`
    : `source:\n  datasource: main\n  sql_query: |\n    SELECT id FROM your_table\nindex:\n  name: ${newName.value}\n  fields:\n    id: { type: keyword, filter: true }\n`
  try {
    await api.createIndex(newName.value, template)
    ElMessage.success('已创建')
    newDialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openSqlTest() {
  sqlResult.value = null
  sqlText.value = ''
  sqlDialog.value = true
}

async function runSqlTest() {
  if (!sqlDS.value || !sqlText.value) return ElMessage.warning('请填写数据源与 SQL')
  sqlLoading.value = true
  try {
    sqlResult.value = await api.sqlTest(sqlDS.value, sqlText.value)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    sqlLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.editor :deep(textarea), .mono :deep(textarea) { font-family: 'Consolas', 'Courier New', monospace; font-size: 14px; line-height: 1.6; }
.sql-editor :deep(textarea) { font-size: 14px; }
</style>
