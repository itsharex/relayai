<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import type { Provider, ProviderUsagePoint, CLIType } from '../stores/app'
import { CLI_TYPES, useAppStore } from '../stores/app'
import CLIIcon from './CLIIcon.vue'
import { maskKey, formatTokens } from '../utils'

const props = defineProps<{
  visible: boolean
  provider: Provider | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
}>()

const store = useAppStore()
const message = useMessage()
const resetLoading = ref(false)
const refreshLoading = ref(false)
const usageSeries = ref<ProviderUsagePoint[]>([])
const loadingSeries = ref(false)
const hoverIndex = ref(-1)
const hoverX = ref(0)
const hoverY = ref(0)
const mouseX = ref(0)
const mouseY = ref(0)
const showPrompt = ref(true)
const showCompletion = ref(true)
const showTotal = ref(true)

const cacheHitRate = computed(() => {
  if (!props.provider || props.provider.prompt_tokens === 0) return 0
  return Math.round((props.provider.cached_tokens / props.provider.prompt_tokens) * 100)
})

const cacheHitRateText = computed(() => {
  if (!props.provider || props.provider.prompt_tokens === 0) return '--'
  return cacheHitRate.value + '%'
})

const cliLabels: Record<string, string> = Object.fromEntries(
  CLI_TYPES.map(t => [t.key, t.label])
)

const cliTypes = computed(() => {
  const types = props.provider?.cli_types
  if (!types || types.length === 0) return []
  return types
})

const chart = computed(() => buildChart(usageSeries.value))

watch(
  () => [props.visible, props.provider?.id, props.provider?.usage_updated_at, props.provider?.total_tokens],
  async () => {
    if (!props.visible || !props.provider) {
      usageSeries.value = []
      return
    }
    showPrompt.value = true
    showCompletion.value = true
    showTotal.value = true
    loadingSeries.value = true
    try {
      usageSeries.value = await store.fetchProviderUsageSeries(props.provider.id)
    } finally {
      loadingSeries.value = false
    }
  },
  { immediate: true }
)

async function handleResetUsage() {
  if (!props.provider) return
  resetLoading.value = true
  try {
    await store.resetProviderUsage(props.provider.id)
    message.success('用量已重置')
  } catch {
    message.error('重置失败')
  } finally {
    resetLoading.value = false
  }
}

async function handleRefreshUsage() {
  refreshLoading.value = true
  try {
    await store.fetchProviders()
    message.success('用量已刷新')
  } catch {
    message.error('刷新失败')
  } finally {
    refreshLoading.value = false
  }
}

function close() {
  emit('update:visible', false)
}

function handleVisibleChange(val: boolean) {
  if (!val) close()
}

function formatDate(value?: number) {
  if (!value) return '暂无'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatDateShort(value?: number) {
  if (!value) return ''
  const d = new Date(value)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function buildChart(points: ProviderUsagePoint[]) {
  const width = 640
  const height = 240
  const padding = { top: 18, right: 24, bottom: 38, left: 50 }
  const innerWidth = width - padding.left - padding.right
  const innerHeight = height - padding.top - padding.bottom
  const maxValue = Math.max(1, ...points.map(p => p.total_tokens || 0))

  const axisBottom = padding.top + innerHeight
  const axisRight = padding.left + innerWidth

  const barWidth = points.length <= 1 ? 14 : Math.max(4, Math.min(14, innerWidth / points.length * 0.2))
  const barMargin = points.length <= 1 ? 0 : barWidth / 2 + 4

  function xPos(index: number) {
    return padding.left + (points.length <= 1 ? innerWidth / 2 : barMargin + (index / (points.length - 1)) * (innerWidth - 2 * barMargin))
  }

  function yPos(value: number) {
    return padding.top + innerHeight - (value / maxValue) * innerHeight
  }

  const totalPts = points.map((p, i) => ({ x: xPos(i), y: yPos(p.total_tokens || 0) }))

  function smoothPath(pts: { x: number; y: number }[]): string {
    if (pts.length === 0) return ''
    if (pts.length === 1) return `M${pts[0].x.toFixed(1)},${pts[0].y.toFixed(1)}`
    if (pts.length === 2) return `M${pts[0].x.toFixed(1)},${pts[0].y.toFixed(1)} L${pts[1].x.toFixed(1)},${pts[1].y.toFixed(1)}`

    const n = pts.length
    const dxs: number[] = []
    const dys: number[] = []
    const ms: number[] = []
    for (let i = 0; i < n - 1; i++) {
      dxs[i] = pts[i + 1].x - pts[i].x
      dys[i] = pts[i + 1].y - pts[i].y
      ms[i] = dxs[i] !== 0 ? dys[i] / dxs[i] : 0
    }

    let d = `M${pts[0].x.toFixed(1)},${pts[0].y.toFixed(1)}`
    for (let i = 0; i < n - 1; i++) {
      const m = ms[i]
      let m0 = i > 0 ? ms[i - 1] : m
      let m1 = i < n - 2 ? ms[i + 1] : m
      if (m0 * m <= 0) m0 = 0
      if (m1 * m <= 0) m1 = 0
      const dx = dxs[i] / 3
      const cx1 = pts[i].x + dx
      const cy1 = Math.min(pts[i].y + m0 * dx, axisBottom)
      const cx2 = pts[i + 1].x - dx
      const cy2 = Math.min(pts[i + 1].y - m1 * dx, axisBottom)
      d += ` C${cx1.toFixed(1)},${cy1.toFixed(1)} ${cx2.toFixed(1)},${cy2.toFixed(1)} ${pts[i + 1].x.toFixed(1)},${pts[i + 1].y.toFixed(1)}`
    }
    return d
  }

  const bars = points.map((p, i) => {
    const cx = xPos(i)
    const totalVal = p.total_tokens || 0
    const promptVal = p.prompt_tokens || 0
    const completionVal = p.completion_tokens || 0
    const totalH = totalVal / maxValue * innerHeight
    const promptH = totalVal > 0 ? (promptVal / totalVal) * totalH : 0
    const completionH = totalVal > 0 ? (completionVal / totalVal) * totalH : 0
    return {
      cx,
      x: cx - barWidth / 2,
      barWidth,
      completionY: axisBottom - completionH,
      completionH,
      promptY: axisBottom - completionH - promptH,
      promptH,
    }
  })

  const xLabels: { x: number; label: string }[] = []
  if (points.length > 0) {
    xLabels.push({ x: xPos(0), label: formatXLabel(points[0].time) })
    if (points.length > 2) {
      const mid = Math.floor(points.length / 2)
      xLabels.push({ x: xPos(mid), label: formatXLabel(points[mid].time) })
    }
    if (points.length > 1) {
      xLabels.push({ x: xPos(points.length - 1), label: formatXLabel(points[points.length - 1].time) })
    }
  }

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map(r => ({
    y: axisBottom - r * innerHeight,
    value: Math.round(maxValue * r),
    label: formatTokens(Math.round(maxValue * r)),
  }))

  const hitTargets = points.map((p, i) => ({
    x: xPos(i),
    y: yPos(p.total_tokens || 0),
    index: i,
    time: p.time,
    prompt_tokens: p.prompt_tokens || 0,
    completion_tokens: p.completion_tokens || 0,
    total_tokens: p.total_tokens || 0,
    cached_tokens: p.cached_tokens || 0,
    cached_rate: p.prompt_tokens > 0 ? Math.round((p.cached_tokens || 0) / p.prompt_tokens * 100) : 0,
  }))

  return { width, height, maxValue, padding, axisBottom, axisRight, hasData: points.length > 0, totalPath: smoothPath(totalPts), bars, xLabels, yTicks, hitTargets, barWidth }
}

function formatXLabel(time: number): string {
  const d = new Date(time)
  return `${(d.getMonth() + 1).toString().padStart(2, '0')}/${d.getDate().toString().padStart(2, '0')} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function handleChartMouseMove(e: MouseEvent) {
  const container = e.currentTarget as HTMLElement
  const svg = container.querySelector('svg') as SVGSVGElement | null
  if (!svg) return

  const containerRect = container.getBoundingClientRect()
  mouseX.value = e.clientX - containerRect.left
  mouseY.value = e.clientY - containerRect.top

  const ctm = svg.getScreenCTM()
  if (!ctm) return
  const invCTM = ctm.inverse()
  const svgX = invCTM.a * e.clientX + invCTM.c * e.clientY + invCTM.e

  const targets = chart.value.hitTargets
  if (targets.length === 0) { hoverIndex.value = -1; return }

  let closest = 0
  let minDx = Math.abs(targets[0].x - svgX)
  for (let i = 1; i < targets.length; i++) {
    const dx = Math.abs(targets[i].x - svgX)
    if (dx < minDx) { minDx = dx; closest = i }
  }
  hoverIndex.value = closest
  hoverX.value = targets[closest].x
  hoverY.value = targets[closest].y
}

function handleChartMouseLeave() {
  hoverIndex.value = -1
}
</script>

<template>
  <n-modal
    :show="visible"
    preset="card"
    :title="provider ? provider.name : '提供商详情'"
    :style="{ width: 'min(860px, 92vw)', zIndex: 2000, transform: 'translateY(42px)' }"
    :bordered="false"
    @update:show="handleVisibleChange"
  >
    <div v-if="provider" class="details-root">
      <!-- Header: CLI badge + status + info -->
      <div class="details-header">
        <div class="header-top">
          <div class="cli-badges">
            <span v-for="t in cliTypes" :key="t" class="cli-badge" :class="`cli-${t}`">
              <CLIIcon :type="t as CLIType" :size="12" />
              <span>{{ cliLabels[t] || t }}</span>
            </span>
            <span v-if="cliTypes.length === 0" class="cli-badge cli-unknown">未选择</span>
          </div>
          <n-tag :type="provider.enabled ? 'success' : 'default'" size="small" round>
            {{ provider.enabled ? '已启用' : '已禁用' }}
          </n-tag>
          <n-tag v-if="cliTypes.includes('codex') && provider.chat_compat_mode" size="small" type="info" round>
            Chat 兼容
          </n-tag>
        </div>
        <div class="header-info-grid">
          <div class="info-row">
            <span class="info-label">Base URL</span>
            <span class="info-value mono">{{ provider.base_url }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">API Key</span>
            <span class="info-value mono">{{ maskKey(provider.api_key) }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">默认模型</span>
            <span class="info-value">{{ provider.default_model || '未设置' }}</span>
          </div>
          <div class="info-row" v-if="provider.model_mappings?.length">
            <span class="info-label">模型映射</span>
            <span class="info-value">
              <n-tag v-for="m in provider.model_mappings" :key="`${m.from}-${m.to}`" size="tiny" type="info">
                {{ m.from }} → {{ m.to }}
              </n-tag>
            </span>
          </div>
          <div class="info-row">
            <span class="info-label">ID</span>
            <span class="info-value mono" style="font-size: 11px">{{ provider.id }}</span>
          </div>
          <div class="info-row">
            <span class="info-label">创建时间</span>
            <span class="info-value">{{ formatDate(provider.created_at ? provider.created_at * 1000 : 0) }}</span>
          </div>
        </div>
      </div>

      <!-- Stats cards -->
      <div class="stats-header">
        <span class="stats-title">用量统计</span>
        <div class="usage-actions">
          <n-button size="small" :loading="refreshLoading" @mousedown.prevent @click="handleRefreshUsage">
            <template #icon>
              <n-icon><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/></svg></n-icon>
            </template>
            刷新
          </n-button>
          <n-popconfirm @positive-click="handleResetUsage">
            <template #trigger>
              <n-button size="small" type="error" :loading="resetLoading">
                <template #icon>
                  <n-icon><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg></n-icon>
                </template>
                重置
              </n-button>
            </template>
            确认重置该提供商的所有用量统计？
          </n-popconfirm>
        </div>
      </div>
      <div class="stats-grid">
        <div class="stat-card">
          <span class="stat-label">输入用量</span>
          <span class="stat-value">{{ formatTokens(provider.prompt_tokens) }}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">输出用量</span>
          <span class="stat-value">{{ formatTokens(provider.completion_tokens) }}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">总用量</span>
          <span class="stat-value">{{ formatTokens(provider.total_tokens) }}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">缓存命中率</span>
          <span class="stat-value" :class="{ 'cache-good': cacheHitRate >= 50, 'cache-warn': cacheHitRate > 0 && cacheHitRate < 50 }">{{ cacheHitRateText }}</span>
          <span v-if="provider.cached_tokens > 0" class="stat-sub">缓存 {{ formatTokens(provider.cached_tokens) }}</span>
        </div>
      </div>

      <!-- Usage chart -->
      <div class="chart-section">
        <div class="chart-header">
          <span class="chart-title">用量曲线</span>
          <div class="chart-legend">
            <span class="legend-item" :class="{ dimmed: !showPrompt }" @click="showPrompt = !showPrompt">
              <span class="legend-dot" style="background: var(--app-success)"></span>输入
            </span>
            <span class="legend-item" :class="{ dimmed: !showCompletion }" @click="showCompletion = !showCompletion">
              <span class="legend-dot" style="background: var(--app-warning)"></span>输出
            </span>
            <span class="legend-item" :class="{ dimmed: !showTotal }" @click="showTotal = !showTotal">
              <span class="legend-line" style="background: var(--app-danger)"></span>总量
            </span>
          </div>
          <span class="chart-time">最后更新：{{ formatDate(provider.usage_updated_at) }}</span>
        </div>
        <n-spin :show="loadingSeries">
          <div class="chart-container" @mousemove="handleChartMouseMove" @mouseleave="handleChartMouseLeave">
            <svg :viewBox="`0 0 ${chart.width} ${chart.height}`" style="width: 100%; height: 240px; display: block">
              <line v-for="(tick, i) in chart.yTicks" :key="'yg'+i" :x1="chart.padding.left" :y1="tick.y" :x2="chart.axisRight" :y2="tick.y" stroke="var(--app-border-2)" stroke-dasharray="4 4" :stroke-opacity="i === chart.yTicks.length - 1 ? 0 : 0.6" />
              <template v-if="chart.hasData">
                <rect v-if="showCompletion" v-for="(bar, i) in chart.bars" :key="'bc'+i" :x="bar.x" :y="bar.completionY" :width="bar.barWidth" :height="bar.completionH" fill="var(--app-warning)" rx="1" opacity="0.85" />
                <rect v-if="showPrompt" v-for="(bar, i) in chart.bars" :key="'bp'+i" :x="bar.x" :y="bar.promptY" :width="bar.barWidth" :height="bar.promptH" fill="var(--app-success)" rx="1" opacity="0.85" />
              </template>
              <path v-if="chart.hasData && showTotal" :d="chart.totalPath" fill="none" stroke="var(--app-danger)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
              <line :x1="chart.padding.left" :y1="chart.padding.top" :x2="chart.padding.left" :y2="chart.axisBottom" stroke="var(--app-border)" />
              <line :x1="chart.padding.left" :y1="chart.axisBottom" :x2="chart.axisRight" :y2="chart.axisBottom" stroke="var(--app-border)" />
              <text v-for="(tick, i) in chart.yTicks" :key="'yl'+i" :x="chart.padding.left - 8" :y="tick.y + 4" fill="currentColor" font-size="10" text-anchor="end">{{ tick.label }}</text>
              <text v-for="(label, i) in chart.xLabels" :key="'xl'+i" :x="label.x" :y="chart.axisBottom + 16" fill="currentColor" font-size="10" text-anchor="middle">{{ label.label }}</text>
              <rect :x="chart.padding.left" :y="chart.padding.top" :width="chart.axisRight - chart.padding.left" :height="chart.axisBottom - chart.padding.top" fill="transparent" stroke="none" style="cursor: crosshair" />
              <line v-if="hoverIndex >= 0 && chart.hasData" :x1="hoverX" :y1="chart.padding.top" :x2="hoverX" :y2="chart.axisBottom" stroke="var(--app-text-3)" stroke-width="1" stroke-dasharray="3 3" />
              <circle v-if="hoverIndex >= 0 && chart.hasData" :cx="hoverX" :cy="hoverY" r="4" fill="var(--app-danger)" stroke="#fff" stroke-width="2" />
              <text v-if="!chart.hasData" x="320" y="120" text-anchor="middle" fill="currentColor" font-size="13">暂无曲线数据</text>
            </svg>
            <div v-if="hoverIndex >= 0 && chart.hasData" class="chart-tooltip" :style="{ left: mouseX + 'px', top: (mouseY - 12) + 'px', transform: 'translate(-50%, -100%)' }">
              <div class="chart-tooltip-time">{{ formatDateShort(chart.hitTargets[hoverIndex].time) }}</div>
              <div class="chart-tooltip-row"><span class="chart-tooltip-dot" style="background: var(--app-success)"></span><span>输入</span><span class="chart-tooltip-val">{{ formatTokens(chart.hitTargets[hoverIndex].prompt_tokens) }}</span></div>
              <div class="chart-tooltip-row"><span class="chart-tooltip-dot" style="background: var(--app-warning)"></span><span>输出</span><span class="chart-tooltip-val">{{ formatTokens(chart.hitTargets[hoverIndex].completion_tokens) }}</span></div>
              <div class="chart-tooltip-row"><span class="chart-tooltip-dot" style="background: var(--app-danger)"></span><span>总量</span><span class="chart-tooltip-val">{{ formatTokens(chart.hitTargets[hoverIndex].total_tokens) }}</span></div>
              <div class="chart-tooltip-row"><span class="chart-tooltip-dot" style="background: #2080f0"></span><span>缓存</span><span class="chart-tooltip-val">{{ chart.hitTargets[hoverIndex].cached_tokens > 0 ? formatTokens(chart.hitTargets[hoverIndex].cached_tokens) + ' (' + chart.hitTargets[hoverIndex].cached_rate + '%)' : '--' }}</span></div>
            </div>
          </div>
        </n-spin>
      </div>
    </div>
  </n-modal>
</template>

<style scoped>
.details-root {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ---- Header ---- */
.details-header {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.header-top {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.cli-badges {
  display: flex;
  gap: 6px;
}

.cli-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.6;
  border-radius: 12px;
  white-space: nowrap;
}

.cli-badge.cli-claude {
  color: #D97757;
  background: rgba(217, 119, 87, 0.1);
  border: 1px solid rgba(217, 119, 87, 0.2);
}

.cli-badge.cli-codex {
  color: #10B981;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.cli-badge.cli-unknown {
  color: var(--app-text-3);
  background: var(--app-fill-1);
  border: 1px solid var(--app-border-2);
}

.header-info-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 4px;
  padding: 10px 12px;
  background: var(--app-fill-1);
  border-radius: 8px;
  border: 1px solid var(--app-border-2);
}

.info-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 26px;
}

.info-label {
  flex-shrink: 0;
  width: 64px;
  font-size: 12px;
  color: var(--app-text-3);
  text-align: right;
}

.info-value {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--app-text-2);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.info-value.mono {
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  word-break: break-all;
}

/* ---- Stats ---- */
.stats-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.stats-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--app-text-2);
}

.usage-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 14px 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  text-align: center;
}

.stat-label {
  font-size: 12px;
  color: var(--app-text-3);
}

.stat-value {
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.stat-value.cache-good { color: var(--app-success); }
.stat-value.cache-warn { color: var(--app-warning); }

.stat-sub {
  font-size: 11px;
  color: var(--app-text-3);
}

/* ---- Chart ---- */
.chart-section {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 12px;
}

.chart-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.chart-title {
  font-weight: 600;
  font-size: 14px;
  flex-shrink: 0;
}

.chart-legend {
  display: flex;
  gap: 14px;
  align-items: center;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--app-text-3);
  cursor: pointer;
  user-select: none;
  transition: opacity 0.15s;
}

.legend-item.dimmed { opacity: 0.3; }

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
  flex-shrink: 0;
}

.legend-line {
  width: 16px;
  height: 2px;
  border-radius: 1px;
  flex-shrink: 0;
}

.chart-time {
  margin-left: auto;
  font-size: 11px;
  color: var(--app-text-3);
}

.chart-container {
  width: 100%;
  overflow: hidden;
  position: relative;
}

/* ---- Tooltip ---- */
.chart-tooltip {
  position: absolute;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.6;
  pointer-events: none;
  white-space: nowrap;
  z-index: 10;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

:root.dark .chart-tooltip {
  background: rgba(36, 36, 40, 0.95);
}

.chart-tooltip-time {
  font-weight: 600;
  color: var(--app-text-2);
  margin-bottom: 2px;
  font-size: 11px;
}

.chart-tooltip-row {
  display: flex;
  align-items: center;
  gap: 5px;
  color: var(--app-text-3);
  font-size: 11px;
}

.chart-tooltip-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.chart-tooltip-val {
  font-family: 'SF Mono', 'Menlo', 'Consolas', monospace;
  font-variant-numeric: tabular-nums;
  color: var(--app-text-2);
  margin-left: auto;
  padding-left: 6px;
}

/* ---- Responsive ---- */
@media (max-width: 600px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .header-info-grid {
    padding: 8px 10px;
  }
  .info-label {
    width: 56px;
    font-size: 11px;
  }
}
</style>
