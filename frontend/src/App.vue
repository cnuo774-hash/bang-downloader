<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { backend, type PublicConfig, type Task, type TaskEvent } from './api'
import { TaskStore } from './taskStore'

const store = reactive(new TaskStore())
const tasks = computed(() => store.tasks)
const config = ref<PublicConfig>({ output: '', maxDownload: '', engineVersion: '1.37.0' })
const activeView = ref<'tasks' | 'settings'>('tasks')
const filter = ref<'all' | 'active' | 'complete'>('all')
const showAdd = ref(false)
const source = ref('')
const output = ref('')
const busy = ref(false)
const notice = ref('')
const taskTotal = computed(() => store.total)
const loadedOffset = computed(() => store.tasks.length)
const loadingMore = ref(false)
let unsubscribe: undefined | (() => void)
let noticeTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const pageSize = 100

const visibleTasks = computed(() => tasks.value.filter((task) => {
  if (filter.value === 'active') return ['active', 'waiting', 'paused'].includes(task.status)
  if (filter.value === 'complete') return task.status === 'complete'
  return true
}))
const activeCount = computed(() => store.stats.active)
const completeCount = computed(() => store.stats.complete)
const totalSpeed = computed(() => store.stats.downloadBps)
const canLoadMore = computed(() => loadedOffset.value < taskTotal.value)

onMounted(async () => {
  try {
    unsubscribe = backend.onTask(upsert)
    config.value = await backend.config()
    output.value = config.value.output
    await reload()
  } catch (error) {
    showError(error)
  }
})

onUnmounted(() => { disposed = true; unsubscribe?.(); clearTimeout(noticeTimer) })

function upsert(event: TaskEvent) {
  if (!disposed) store.apply(event)
}

async function reload() {
  for (let attempt = 0; attempt < 3; attempt++) {
    const page = await backend.list(0, pageSize)
    if (disposed || store.page(page)) return
  }
  throw new Error('任务列表正在变化，请稍后重试')
}

async function loadMore() {
  if (!canLoadMore.value || loadingMore.value) return
  loadingMore.value = true
  try {
    const page = await backend.list(loadedOffset.value, pageSize)
    if (!disposed && !store.page(page, true)) await reload()
  } catch (error) {
    showError(error)
  } finally {
    loadingMore.value = false
  }
}

async function submit() {
  if (!source.value.trim() || !output.value.trim()) return
  busy.value = true
  try {
    await backend.add(source.value, output.value)
    await reload()
    source.value = ''
    showAdd.value = false
    flash('任务已加入下载队列')
  } catch (error) {
    showError(error)
  } finally {
    busy.value = false
  }
}

async function pickTorrent() {
  try {
    const path = await backend.chooseTorrent()
    if (path) source.value = path
  } catch (error) { showError(error) }
}

async function pickOutput(target = output) {
  try {
    const path = await backend.chooseOutput()
    if (path) target.value = path
  } catch (error) { showError(error) }
}

async function pickSettingsOutput() {
  try {
    const path = await backend.chooseOutput()
    if (path) config.value.output = path
  } catch (error) { showError(error) }
}

async function openOutput() {
  try { await backend.openOutput() } catch (error) { showError(error) }
}

async function saveSettings() {
  busy.value = true
  try {
    await backend.saveSettings(config.value.output, config.value.maxDownload)
    config.value = await backend.config()
    output.value = config.value.output
    flash('设置已保存')
  } catch (error) {
    showError(error)
  } finally {
    busy.value = false
  }
}

async function taskAction(task: Task) {
  try {
    if (task.status === 'paused') await backend.resume(task.id)
    else await backend.pause(task.id)
  } catch (error) {
    showError(error)
  }
}

async function remove(task: Task) {
  try {
    await backend.remove(task.id)
    await reload()
  } catch (error) {
    showError(error)
  }
}

function progress(task: Task) {
  return task.total > 0 ? Math.min(100, task.completed / task.total * 100) : 0
}

function bytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** unit).toFixed(unit ? 1 : 0)} ${units[unit]}`
}

function statusLabel(status: string) {
  return ({ active: '下载中', waiting: '等待中', paused: '已暂停', complete: '已完成', error: '失败', removed: '已移除' } as Record<string, string>)[status] || status
}

function flash(message: string) {
  notice.value = message
  clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => { notice.value = '' }, 5000)
}

function showError(error: unknown) {
  flash(typeof error === 'string' ? error : error instanceof Error ? error.message : '操作失败，请查看日志')
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark">B</span><div><strong>Bang</strong><small>Downloader</small></div></div>
      <nav>
        <button :class="{ selected: activeView === 'tasks' }" @click="activeView = 'tasks'"><span>⌁</span> 下载任务</button>
        <button :class="{ selected: activeView === 'settings' }" @click="activeView = 'settings'"><span>⌘</span> 偏好设置</button>
      </nav>
      <div class="engine-card">
        <span class="online-dot"></span>
        <div><strong>下载引擎</strong><small>aria2 {{ config.engineVersion }}</small></div>
      </div>
    </aside>

    <main v-if="activeView === 'tasks'">
      <header class="topbar">
        <div><p class="eyebrow">DOWNLOAD CENTER</p><h1>我的下载</h1></div>
        <button class="primary" @click="showAdd = true"><span>＋</span> 新建任务</button>
      </header>

      <section class="stats">
        <article><span class="stat-icon amber">↓</span><div><small>当前速度</small><strong>{{ bytes(totalSpeed) }}/s</strong></div></article>
        <article><span class="stat-icon green">◌</span><div><small>下载中</small><strong>{{ activeCount }} <em>项</em></strong></div></article>
        <article><span class="stat-icon ink">✓</span><div><small>已完成</small><strong>{{ completeCount }} <em>项</em></strong></div></article>
      </section>

      <section class="task-panel">
        <div class="panel-head">
          <div class="tabs">
            <button v-for="item in ['all', 'active', 'complete']" :key="item" :class="{ active: filter === item }" @click="filter = item as typeof filter">
              {{ item === 'all' ? '全部' : item === 'active' ? '进行中' : '已完成' }}
            </button>
          </div>
          <button class="ghost" @click="openOutput">打开下载目录 ↗</button>
        </div>

        <div v-if="!visibleTasks.length" class="empty">
          <span>↓</span><h3>下载列表还是空的</h3><p>添加磁力链接、HTTP 地址或本地种子文件。</p>
          <button class="secondary" @click="showAdd = true">添加第一个任务</button>
          <button v-if="canLoadMore" class="load-more" :disabled="loadingMore" @click="loadMore">加载更多任务</button>
        </div>
        <DynamicScroller v-else class="task-list" :items="visibleTasks" :min-item-size="112" key-field="id" @scroll-end="loadMore">
          <template #default="{ item, active }">
            <DynamicScrollerItem :item="item" :active="active" :size-dependencies="[item.completed, item.status]">
              <article class="task-row">
                <div class="file-icon">{{ item.status === 'complete' ? '✓' : '↓' }}</div>
                <div class="task-content">
                  <div class="task-title"><strong>{{ item.name }}</strong><span :class="['badge', item.status]">{{ statusLabel(item.status) }}</span></div>
                  <div class="progress"><i :style="{ width: `${progress(item)}%` }"></i></div>
                  <div class="task-meta"><span>{{ bytes(item.completed) }} / {{ bytes(item.total) }}</span><span v-if="item.status === 'active'">{{ bytes(item.downloadBps) }}/s</span><span class="path">{{ item.output }}</span></div>
                  <p v-if="item.error" class="task-error">{{ item.error }}</p>
                </div>
                <div class="actions">
                  <button v-if="!['complete', 'error', 'removed'].includes(item.status)" :aria-label="item.status === 'paused' ? '恢复任务' : '暂停任务'" @click="taskAction(item)">{{ item.status === 'paused' ? '▶' : 'Ⅱ' }}</button>
                  <button aria-label="移除任务" title="移除记录，保留已下载文件" @click="remove(item)">×</button>
                </div>
              </article>
            </DynamicScrollerItem>
          </template>
          <template #after>
            <button v-if="canLoadMore" class="load-more" :disabled="loadingMore" @click="loadMore">
              {{ loadingMore ? '正在加载…' : '加载更多任务' }}
            </button>
          </template>
        </DynamicScroller>
      </section>
    </main>

    <main v-else>
      <header class="topbar"><div><p class="eyebrow">PREFERENCES</p><h1>偏好设置</h1></div></header>
      <section class="settings-card">
        <div><h2>下载位置</h2><p>新任务默认保存到这里。支持绝对路径、相对路径和 <code>~</code>。</p></div>
        <label>默认保存目录<div class="input-row"><input v-model="config.output" /><button @click="pickSettingsOutput">选择</button></div></label>
        <label>全局下载限速<input v-model="config.maxDownload" class="setting-input" placeholder="留空表示不限速，例如 10M" /></label>
        <div class="security-note"><strong>本地优先，默认安全</strong><p>RPC 仅监听 127.0.0.1，并使用首次启动时生成的随机密钥。密钥不会显示在界面中。</p></div>
        <button class="primary save" :disabled="busy" @click="saveSettings">保存设置</button>
      </section>
    </main>

    <div v-if="showAdd" class="backdrop" @click.self="showAdd = false">
      <form class="drawer" role="dialog" aria-modal="true" aria-label="添加下载任务" @keydown.esc="showAdd = false" @submit.prevent="submit">
        <div class="drawer-head"><div><p class="eyebrow">NEW DOWNLOAD</p><h2>添加下载任务</h2></div><button type="button" class="close" @click="showAdd = false">×</button></div>
        <label>磁力链接、HTTP 地址或种子文件<textarea v-model="source" rows="5" placeholder="magnet:?xt=urn:btih:…"></textarea></label>
        <button type="button" class="file-pick" @click="pickTorrent">选择本地 .torrent 文件</button>
        <label>保存到<div class="input-row"><input v-model="output" /><button type="button" @click="pickOutput()">选择</button></div></label>
        <div class="drawer-actions"><button type="button" class="ghost" @click="showAdd = false">取消</button><button class="primary" :disabled="busy || !source.trim()">{{ busy ? '正在添加…' : '开始下载' }}</button></div>
      </form>
    </div>
    <Transition name="toast"><div v-if="notice" class="toast" role="status">{{ notice }}</div></Transition>
  </div>
</template>
