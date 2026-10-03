export type Task = {
  id: string
  name: string
  status: string
  total: number
  completed: number
  downloadBps: number
  output: string
  error?: string
  updatedAt: number
  addedAt: number
  followedBy?: string[]
}

export type TaskEvent = {
  kind: 'upsert' | 'remove'
  task: Task
  added: boolean
  total: number
  revision: number
  stats?: TaskStats
}
export type TaskStats = { active: number; complete: number; downloadBps: number }
export type TaskPage = { items: Task[]; total: number; revision: number; stats?: TaskStats }

// Keep a contiguous prefix of the backend's stable ordering. Updates to unloaded
// tasks do not insert duplicate rows or advance the next page's offset.
export class TaskStore {
  tasks: Task[] = []
  total = 0
  revision = 0
  initialized = false
  stats: TaskStats = { active: 0, complete: 0, downloadBps: 0 }
  private pending: TaskEvent[] = []

  apply(event: TaskEvent) {
    if (!this.initialized) {
      this.pending.push(event)
      return
    }
    if (event.revision <= this.revision) return
    const index = this.tasks.findIndex((task) => task.id === event.task.id)
    const fullyLoaded = this.tasks.length >= this.total
    if (event.kind === 'remove') {
      if (index >= 0) this.tasks.splice(index, 1)
    } else if (index >= 0) this.tasks[index] = event.task
    else if (event.added || fullyLoaded) { this.tasks.push(event.task); this.sort() }
    if (event.stats) this.stats = event.stats
    this.total = event.total
    this.revision = event.revision
  }

  page(page: TaskPage, append = false): boolean {
    if (this.initialized && (page.revision < this.revision || (append && page.revision !== this.revision))) return false
    if (append) {
      const known = new Set(this.tasks.map((task) => task.id))
      this.tasks.push(...page.items.filter((task) => !known.has(task.id)))
    } else this.tasks = page.items || []
    this.total = page.total
    this.revision = page.revision
    if (page.stats) this.stats = page.stats
    this.initialized = true
    this.sort()
    for (const event of this.pending) this.apply(event)
    this.pending = []
    return true
  }

  private sort() {
    this.tasks.sort((a, b) => b.addedAt - a.addedAt || (a.id < b.id ? 1 : a.id > b.id ? -1 : 0))
  }
}
