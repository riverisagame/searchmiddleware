<template>
  <div>
    <el-card>
      <template #header>搜索测试</template>
      <el-form inline @submit.prevent>
        <el-form-item label="索引">
          <el-select v-model="form.index" style="width: 180px" placeholder="选择索引">
            <el-option v-for="i in indexes" :key="i" :label="i" :value="i" />
          </el-select>
        </el-form-item>
        <el-form-item label="关键词">
          <el-input v-model="form.keyword" placeholder="留空 = match_all" style="width: 220px" clearable @keyup.enter="search" />
        </el-form-item>
        <el-form-item label="site_id">
          <el-input v-model="form.site_id" placeholder="可选" style="width: 100px" clearable />
        </el-form-item>
        <el-form-item label="排序">
          <el-select v-model="form.sort" style="width: 150px" clearable>
            <el-option label="相关度" value="score" />
            <el-option label="价格↓" value="price:desc" />
            <el-option label="价格↑" value="price:asc" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="form.highlight">高亮</el-checkbox>
        </el-form-item>
        <el-button type="primary" size="default" @click="search" :loading="searching">🔍 搜索</el-button>
        <el-button v-if="result" @click="resetSearch">清空</el-button>
      </el-form>

      <el-collapse v-model="advOpen">
        <el-collapse-item name="adv" title="高级选项（filter / aggs JSON）">
          <el-form label-width="90px">
            <el-form-item label="filter JSON">
              <el-input v-model="form.filter" class="mono" placeholder='{"status":1,"category_ids":[238],"price":{"gte":10,"lte":100}}' />
            </el-form-item>
            <el-form-item label="aggs JSON">
              <el-input v-model="form.aggs" class="mono" placeholder='{"categories":{"field":"category_ids","size":20},"price_ranges":{"field":"price","ranges":[[0,100],[100,300]]}}' />
            </el-form-item>
          </el-form>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <el-card v-if="result" style="margin-top: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span>结果：{{ result.total }} 条（{{ result.took }}ms）</span>
          <div style="display: flex; align-items: center; gap: 10px">
            <el-button v-if="result.items?.length" size="small" @click="exportCsv">导出 CSV</el-button>
            <el-radio-group v-model="viewMode" size="small">
              <el-radio-button value="table">表格</el-radio-button>
              <el-radio-button value="json">JSON</el-radio-button>
            </el-radio-group>
          </div>
        </div>
      </template>

      <template v-if="viewMode === 'table'">
        <el-table stripe :data="result.items" size="small" border>
          <el-table-column label="ID" width="80">
            <template #default="{ row }">{{ row.id }}</template>
          </el-table-column>
          <el-table-column label="分数" width="90">
            <template #default="{ row }">{{ Number(row.score).toFixed(4) }}</template>
          </el-table-column>
          <el-table-column label="字段">
            <template #default="{ row }">
              <div v-for="(v, k) in row.fields" :key="k" class="field-row">
                <span class="field-key">{{ k }}:</span>
                <span v-if="row.highlight && row.highlight[k]"
                  v-html="Array.isArray(row.highlight[k]) ? row.highlight[k][0] : row.highlight[k]" />
                <span v-else>{{ fmt(v) }}</span>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </template>

      <pre v-else class="json-view mono">{{ resultJson }}</pre>

      <div v-if="result.aggs" style="margin-top: 16px">
        <el-divider>聚合</el-divider>
        <el-row :gutter="16">
          <el-col v-for="(agg, name) in result.aggs" :key="name" :span="12">
            <el-card shadow="never" size="small" style="margin-bottom: 12px">
              <template #header>{{ name }}<span class="drill-tip">（点击 bucket 下钻过滤）</span></template>
              <div v-for="b in agg.buckets" :key="String(b.key)" class="bucket-row clickable" @click="drillDown(name, b)">
                <span>{{ b.key }}</span>
                <el-progress :percentage="bucketPct(b.doc_count)" :format="() => b.doc_count" :stroke-width="14" />
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const indexes = ref([])
const result = ref(null)
const viewMode = ref('table')
const advOpen = ref([])
const searching = ref(false)
const resultJson = computed(() => (result.value ? JSON.stringify(result.value, null, 2) : ''))
const form = reactive({
  index: '',
  keyword: '',
  site_id: '',
  sort: 'score',
  highlight: false,
  filter: '',
  aggs: '',
})

function fmt(v) {
  if (v === null || v === undefined) return ''
  if (Array.isArray(v)) return v.join(', ')
  return String(v)
}

function bucketPct(n) {
  const max = Math.max(...Object.values(result.value?.aggs || {}).flatMap((a) => a.buckets.map((b) => b.doc_count)))
  return max ? Math.round((n / max) * 100) : 0
}

// 下钻：点击 bucket → 从 aggs JSON 解析字段名 → 更新 filter（覆盖同名过滤）→ 重新搜索
function drillDown(aggName, bucket) {
  let field = null
  try {
    const aggs = JSON.parse(form.aggs || '{}')
    field = aggs[aggName]?.field || null
  } catch { /* aggs JSON 解析失败则忽略 */ }
  if (!field) {
    ElMessage.warning('无法解析该聚合的字段名（检查 aggs JSON）')
    return
  }
  const key = typeof bucket.key === 'string' ? bucket.key : String(bucket.key)
  let filter = {}
  try { filter = form.filter ? JSON.parse(form.filter) : {} } catch { /* 无效 filter 重置 */ }
  filter[field] = [key] // 覆盖该字段过滤
  form.filter = JSON.stringify(filter)
  search()
}

async function search() {
  if (!form.index) return ElMessage.warning('请选择索引')
  const params = { index: form.index, limit: 20 }
  if (form.keyword) params.keyword = form.keyword
  if (form.site_id) params.site_id = form.site_id
  if (form.sort && form.sort !== 'score') params.sort = form.sort
  if (form.highlight) params.highlight = '1'
  if (form.filter) params.filter = form.filter
  if (form.aggs) params.aggs = form.aggs

  searching.value = true
  try {
    const data = await api.search(params)
    result.value = data
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    searching.value = false
  }
}

function resetSearch() {
  result.value = null
  form.keyword = ''
  form.site_id = ''
  form.sort = 'score'
  form.highlight = false
  form.filter = ''
  form.aggs = ''
}

// 导出结果为 CSV
function exportCsv() {
  const items = result.value?.items || []
  if (!items.length) return ElMessage.warning('无数据可导出')
  const headers = ['id', 'score']
  const fields = new Set()
  for (const it of items) for (const k of Object.keys(it.fields || {})) fields.add(k)
  headers.push(...fields)
  const esc = (v) => {
    const s = String(v ?? '')
    return /[",\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
  }
  const lines = [headers.join(',')]
  for (const it of items) {
    const row = [it.id, it.score, ...[...fields].map((f) => esc((it.fields || {})[f]))]
    lines.push(row.join(','))
  }
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `search_${form.index || 'result'}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  indexes.value = (await api.listIndexes().catch(() => [])) || []
})
</script>

<style scoped>
.field-row { line-height: 1.6; }
.field-key { color: #909399; margin-right: 6px; }
.bucket-row { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.bucket-row span { min-width: 80px; }
.clickable { cursor: pointer; }
.clickable:hover { background: #f5f7fa; border-radius: 4px; }
.drill-tip { color: #909399; font-size: 12px; font-weight: normal; margin-left: 8px; }
.json-view {
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 12px;
  max-height: 420px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.5;
}
</style>
