<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onUnmounted,
  reactive,
  ref,
  watch
} from 'vue'
import { backend, type PublicConfig, type Task, type TaskEvent } from './api'
import { TaskStore } from './taskStore'
import AppIcon from './components/AppIcon.vue'
import hitoriPortrait from './assets/bocchi/hitori-main.png'
import hitoriExpressions from './assets/bocchi/hitori-icon.png'

const store = reactive(new TaskStore())
const tasks = computed(() => store.tasks)
const config = ref<PublicConfig>({
  output: '',
  maxDownload: '',
  engineVersion: '1.37.0'
})
const activeView = ref<'tasks' | 'settings'>('tasks')
const filter = ref<'all' | 'active' | 'complete'>('all')
const showAdd = ref(false)
const source = ref('')
const output = ref('')
const busy = ref(false)
const notice = ref('')
const noticeKind = ref<'success' | 'error'>('success')
const connection = ref<'connecting' | 'ready' | 'error'>('connecting')
const dialog = ref<HTMLFormElement>()
const sourceInput = ref<HTMLTextAreaElement>()
let previousFocus: HTMLElement | null = null

watch(showAdd, async (open) => {
  if (open) {
    previousFocus =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null
    await nextTick()
    sourceInput.value?.focus()
  } else {
    await nextTick()
    previousFocus?.focus()
  }
})

function openAdd(event: MouseEvent) {
  if (event.currentTarget instanceof HTMLElement) event.currentTarget.focus()
  showAdd.value = true
}

function trapDialogKeys(event: KeyboardEvent) {
  if (event.key === 'Escape' && !busy.value) {
    showAdd.value = false
    return
  }
  if (event.key !== 'Tab') return
  const controls = dialog.value?.querySelectorAll<HTMLElement>(
    'button:not(:disabled), input, textarea'
  )
  if (!controls?.length) return
  // WebKit may skip buttons with the OS default keyboard navigation setting.
  // Move explicitly so every dialog control is reachable on all platforms.
  event.preventDefault()
  const index = Array.from(controls).findIndex(
    (control) => control === document.activeElement
  )
  const next =
    index < 0
      ? event.shiftKey
        ? controls.length - 1
        : 0
      : (index + (event.shiftKey ? -1 : 1) + controls.length) % controls.length
  controls[next].focus()
}
const taskTotal = computed(() => store.total)
const loadedOffset = computed(() => store.tasks.length)
const loadingMore = ref(false)
let unsubscribe: undefined | (() => void)
let noticeTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const pageSize = 100

const visibleTasks = computed(() =>
  tasks.value.filter((task) => {
    if (filter.value === 'active')
      return ['active', 'waiting', 'paused'].includes(task.status)
    if (filter.value === 'complete') return task.status === 'complete'
    return true
  })
)
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
    connection.value = 'ready'
  } catch (error) {
    connection.value = 'error'
    showError(error)
  }
})

onUnmounted(() => {
  disposed = true
  unsubscribe?.()
  clearTimeout(noticeTimer)
})

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
  } catch (error) {
    showError(error)
  }
}

async function pickOutput(target = output) {
  try {
    const path = await backend.chooseOutput()
    if (path) target.value = path
  } catch (error) {
    showError(error)
  }
}

async function pickSettingsOutput() {
  try {
    const path = await backend.chooseOutput()
    if (path) config.value.output = path
  } catch (error) {
    showError(error)
  }
}

async function openOutput() {
  try {
    await backend.openOutput()
  } catch (error) {
    showError(error)
  }
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
  return task.total > 0 ? Math.min(100, (task.completed / task.total) * 100) : 0
}

function bytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const unit = Math.min(
    Math.floor(Math.log(value) / Math.log(1024)),
    units.length - 1
  )
  return `${(value / 1024 ** unit).toFixed(unit ? 1 : 0)} ${units[unit]}`
}

function statusLabel(status: string) {
  return (
    (
      {
        active: '下载中',
        waiting: '等待中',
        paused: '已暂停',
        complete: '已完成',
        error: '失败',
        removed: '已移除'
      } as Record<string, string>
    )[status] || status
  )
}

function flash(message: string, kind: 'success' | 'error' = 'success') {
  noticeKind.value = kind
  notice.value = message
  clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => {
    notice.value = ''
  }, 5000)
}

function showError(error: unknown) {
  flash(
    typeof error === 'string'
      ? error
      : error instanceof Error
        ? error.message
        : '操作失败，请查看日志',
    'error'
  )
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true"
          >b<span class="hair-clips"><i></i><i></i></span
        ></span>
        <div>
          <strong>Bang<span>.</span></strong
          ><small>BOCCHI EDITION</small>
        </div>
      </div>
      <p class="nav-label">工作空间</p>
      <nav aria-label="主导航">
        <button
          :class="{ selected: activeView === 'tasks' }"
          :aria-current="activeView === 'tasks' ? 'page' : undefined"
          @click="activeView = 'tasks'"
        >
          <AppIcon name="download" /><span>下载任务</span
          ><span v-if="taskTotal" class="nav-count">{{ taskTotal }}</span>
        </button>
        <button
          :class="{ selected: activeView === 'settings' }"
          :aria-current="activeView === 'settings' ? 'page' : undefined"
          @click="activeView = 'settings'"
        >
          <AppIcon name="settings" /><span>偏好设置</span>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <div class="theme-note">
          <span class="theme-note-icon"
            ><AppIcon name="music" :size="18"
          /></span>
          <p>一个人的下载时间。<small>慢慢来，也没关系。</small></p>
          <span class="tiny-clips" aria-hidden="true"><i></i><i></i></span>
        </div>
        <div class="engine-card">
          <span :class="['online-dot', connection]"></span>
          <div>
            <strong>{{
              connection === 'ready'
                ? '引擎已连接'
                : connection === 'error'
                  ? '连接失败'
                  : '正在连接'
            }}</strong
            ><small>aria2 {{ config.engineVersion }}</small>
          </div>
          <span class="engine-local">LOCAL</span>
        </div>
        <p class="edition-label">Bang Downloader <span>2.0.1</span></p>
      </div>
    </aside>

    <main v-if="activeView === 'tasks'" class="downloads-view">
      <header class="topbar">
        <div>
          <p class="eyebrow">YOUR LITTLE DOWNLOAD SPACE</p>
          <h1>我的下载<span class="heading-dot">.</span></h1>
        </div>
        <button
          class="primary"
          :disabled="connection !== 'ready'"
          @click="openAdd"
        >
          <AppIcon name="plus" :size="18" />新建任务
        </button>
      </header>
      <section class="welcome" aria-label="波奇酱主题欢迎区">
        <div class="welcome-copy">
          <span class="theme-tag"
            ><span class="tiny-clips" aria-hidden="true"><i></i><i></i></span>
            BOCCHI THE ROCK!</span
          >
          <h2>让喜欢的，慢慢抵达。</h2>
          <p>链接放进来，剩下的交给 Bang。<br />今天也按自己的节奏来就好。</p>
          <span class="welcome-caption"
            ><AppIcon name="music" :size="14" /> HITORI GOTOH / GUITAR</span
          >
        </div>
        <div class="welcome-art" aria-hidden="true">
          <div class="art-ring"></div>
          <span class="art-word">ぼっち</span><span class="art-spark">✦</span
          ><img :src="hitoriPortrait" alt="" draggable="false" />
        </div>
      </section>
      <section class="stats" aria-label="下载统计">
        <article>
          <span class="stat-icon pink"><AppIcon name="activity" /></span>
          <div>
            <small>当前速度</small
            ><strong>{{ bytes(totalSpeed) }}<em>/s</em></strong>
          </div>
        </article>
        <article>
          <span class="stat-icon blue"><AppIcon name="download" /></span>
          <div>
            <small>下载中</small
            ><strong>{{ activeCount }}<em>项任务</em></strong>
          </div>
        </article>
        <article>
          <span class="stat-icon yellow"><AppIcon name="check" /></span>
          <div>
            <small>已完成</small
            ><strong>{{ completeCount }}<em>项任务</em></strong>
          </div>
        </article>
      </section>
      <section class="task-panel" aria-label="下载任务列表">
        <div class="panel-head">
          <div class="tabs" aria-label="任务筛选">
            <button
              v-for="item in ['all', 'active', 'complete'] as const"
              :key="item"
              :class="{ active: filter === item }"
              :aria-pressed="filter === item"
              @click="filter = item"
            >
              {{
                item === 'all'
                  ? '全部任务'
                  : item === 'active'
                    ? '进行中'
                    : '已完成'
              }}<span v-if="item === 'all'" class="tab-count">{{
                taskTotal
              }}</span>
            </button>
          </div>
          <button class="ghost directory-button" @click="openOutput">
            <AppIcon name="folder" :size="16" />下载目录
          </button>
        </div>
        <div v-if="connection !== 'ready'" class="empty connection-empty">
          <AppIcon
            :name="connection === 'error' ? 'info' : 'activity'"
            :size="28"
          />
          <h3>
            {{
              connection === 'error'
                ? '暂时无法连接下载引擎'
                : '正在读取下载队列'
            }}
          </h3>
          <p>
            {{
              connection === 'error'
                ? '请重新打开应用，或查看本地日志。'
                : '稍等一下，马上就好。'
            }}
          </p>
        </div>
        <div v-else-if="!visibleTasks.length" class="empty">
          <img
            :src="hitoriExpressions"
            alt="波奇酱的两种表情"
            class="empty-character"
            draggable="false"
          />
          <div>
            <h3>
              {{
                filter === 'all'
                  ? '这里还很安静。'
                  : filter === 'complete'
                    ? '还没有已完成的任务'
                    : '暂时没有进行中的任务'
              }}
            </h3>
            <p>添加磁力链接、HTTP 地址或本地种子。</p>
            <button class="secondary" @click="openAdd">
              <AppIcon name="plus" :size="16" />添加任务</button
            ><button
              v-if="canLoadMore"
              class="load-more"
              :disabled="loadingMore"
              @click="loadMore"
            >
              加载更多任务
            </button>
          </div>
        </div>
        <DynamicScroller
          v-else
          class="task-list"
          :items="visibleTasks"
          :min-item-size="110"
          key-field="id"
          @scroll-end="loadMore"
        >
          <template #default="{ item, active }"
            ><DynamicScrollerItem
              :item="item"
              :active="active"
              :size-dependencies="[item.completed, item.status, item.error]"
              ><article class="task-row">
                <div :class="['file-icon', item.status]">
                  <AppIcon
                    :name="item.status === 'complete' ? 'check' : 'file'"
                  />
                </div>
                <div class="task-content">
                  <div class="task-title">
                    <strong :title="item.name">{{ item.name }}</strong
                    ><span :class="['badge', item.status]">{{
                      statusLabel(item.status)
                    }}</span>
                  </div>
                  <div
                    :class="['progress', item.status]"
                    role="progressbar"
                    :aria-label="item.name + ' 下载进度'"
                    :aria-valuenow="
                      item.total > 0 ? Math.round(progress(item)) : undefined
                    "
                    :aria-valuemin="0"
                    :aria-valuemax="100"
                  >
                    <i :style="{ width: `${progress(item)}%` }"></i>
                  </div>
                  <div class="task-meta">
                    <span
                      >{{ bytes(item.completed) }} /
                      {{ bytes(item.total) }}</span
                    ><span v-if="item.status === 'active'" class="task-speed"
                      >{{ bytes(item.downloadBps) }}/s</span
                    ><span class="path" :title="item.output">{{
                      item.output
                    }}</span>
                  </div>
                  <p v-if="item.error" class="task-error">{{ item.error }}</p>
                </div>
                <div class="actions">
                  <button
                    v-if="
                      !['complete', 'error', 'removed'].includes(item.status)
                    "
                    :aria-label="
                      item.status === 'paused' ? '恢复任务' : '暂停任务'
                    "
                    :title="item.status === 'paused' ? '恢复任务' : '暂停任务'"
                    @click="taskAction(item)"
                  >
                    <AppIcon
                      :name="item.status === 'paused' ? 'play' : 'pause'"
                      :size="16"
                    /></button
                  ><button
                    aria-label="移除任务，保留文件"
                    title="移除记录，保留已下载文件"
                    class="remove-button"
                    @click="remove(item)"
                  >
                    <AppIcon name="close" :size="16" />
                  </button>
                </div></article></DynamicScrollerItem
          ></template>
          <template #after
            ><button
              v-if="canLoadMore"
              class="load-more"
              :disabled="loadingMore"
              @click="loadMore"
            >
              {{ loadingMore ? '正在加载…' : '加载更多任务'
              }}<AppIcon name="arrow" :size="14" /></button
          ></template>
        </DynamicScroller>
        <footer class="panel-footer">
          <span><span class="footer-dot"></span>本地下载 · 文件由你保管</span
          ><span>按自己的节奏来。</span>
        </footer>
      </section>
    </main>

    <main v-else class="settings-view">
      <header class="topbar">
        <div>
          <p class="eyebrow">MAKE YOURSELF AT HOME</p>
          <h1>偏好设置<span class="heading-dot">.</span></h1>
        </div>
        <span class="page-chip"
          ><span class="tiny-clips" aria-hidden="true"><i></i><i></i></span
          >你的下载，你的节奏</span
        >
      </header>
      <section class="settings-card">
        <div class="settings-intro">
          <span class="settings-icon"
            ><AppIcon name="settings" :size="24"
          /></span>
          <div>
            <h2>给下载一个小小的家</h2>
            <p>选好目录和速度，下次打开也会记得。</p>
          </div>
        </div>
        <form @submit.prevent="saveSettings">
          <label for="default-output"
            >默认保存目录<span class="field-hint"
              >新建任务时默认使用这个位置。</span
            ></label
          >
          <div class="input-row">
            <input
              id="default-output"
              v-model="config.output"
              autocomplete="off"
            /><button
              type="button"
              class="input-button"
              @click="pickSettingsOutput"
            >
              <AppIcon name="folder" :size="16" />选择目录
            </button>
          </div>
          <label for="download-limit"
            >全局下载限速<span class="field-hint"
              >支持 512K、10M、1G，留空或 0 表示不限速。</span
            ></label
          >
          <div class="limit-input">
            <input
              id="download-limit"
              v-model="config.maxDownload"
              placeholder="不限速"
              autocomplete="off"
            /><span>所有下载任务</span>
          </div>
          <div class="settings-actions">
            <span>设置保存在这台电脑上。</span
            ><button class="primary" :disabled="busy || connection !== 'ready'">
              <AppIcon name="check" :size="16" />{{
                busy ? '正在保存…' : '保存设置'
              }}
            </button>
          </div>
        </form>
      </section>
      <section class="theme-card">
        <img :src="hitoriExpressions" alt="后藤一里" draggable="false" />
        <div>
          <span class="eyebrow">BOCCHI EDITION</span>
          <h3>粉色，是今天的小小勇气。</h3>
          <p>后藤一里主题 · 本地优先的下载空间</p>
          <span class="theme-swatches" aria-label="主题配色：粉、蓝、黄、深灰"
            ><i></i><i></i><i></i><i></i
          ></span>
        </div>
        <span class="theme-credit"
          >角色素材 ©はまじあき／芳文社・アニプレックス</span
        >
      </section>
    </main>

    <Transition name="modal"
      ><div
        v-if="showAdd"
        class="backdrop"
        @click.self="!busy && (showAdd = false)"
      >
        <form
          ref="dialog"
          class="drawer"
          role="dialog"
          aria-modal="true"
          aria-labelledby="add-title"
          @keydown="trapDialogKeys"
          @submit.prevent="submit"
        >
          <div class="drawer-head">
            <span class="dialog-icon"
              ><AppIcon name="download" :size="22" /></span
            ><button
              type="button"
              class="icon-button close"
              aria-label="关闭添加任务窗口"
              :disabled="busy"
              @click="showAdd = false"
            >
              <AppIcon name="close" />
            </button>
          </div>
          <p class="eyebrow">ONE LINK, ONE LITTLE STEP</p>
          <h2 id="add-title">下一份喜欢，从这里开始。</h2>
          <p class="dialog-description">放入下载链接，或选择一个种子文件。</p>
          <label for="download-source">下载来源</label
          ><textarea
            id="download-source"
            ref="sourceInput"
            v-model="source"
            rows="4"
            placeholder="粘贴 magnet:、https:// 链接或种子文件路径"
            required
          ></textarea
          ><button type="button" class="file-pick" @click="pickTorrent">
            <AppIcon name="file" :size="16" />选择 .torrent 文件<AppIcon
              name="arrow"
              :size="14"
            /></button
          ><label for="task-output">保存目录</label>
          <div class="input-row">
            <input
              id="task-output"
              v-model="output"
              required
              autocomplete="off"
            /><button type="button" class="input-button" @click="pickOutput()">
              <AppIcon name="folder" :size="16" />选择
            </button>
          </div>
          <p class="dialog-hint">移除任务时，已经下载的文件会保留。</p>
          <div class="drawer-actions">
            <button
              type="button"
              class="ghost"
              :disabled="busy"
              @click="showAdd = false"
            >
              再想想</button
            ><button
              class="primary"
              :disabled="busy || !source.trim() || !output.trim()"
            >
              {{ busy ? '正在添加…' : '开始下载'
              }}<AppIcon name="arrow" :size="17" />
            </button>
          </div>
        </form></div
    ></Transition>
    <Transition name="toast"
      ><div
        v-if="notice"
        :class="['toast', noticeKind]"
        :role="noticeKind === 'error' ? 'alert' : 'status'"
      >
        <AppIcon
          :name="noticeKind === 'error' ? 'info' : 'check'"
          :size="18"
        /><span>{{ notice }}</span
        ><button class="toast-close" aria-label="关闭提示" @click="notice = ''">
          <AppIcon name="close" :size="14" />
        </button></div
    ></Transition>
  </div>
</template>
