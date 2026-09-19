import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime'
import {
  Add,
  ChooseOutput,
  ChooseTorrent,
  Config,
  ListTasks,
  OpenOutput,
  Pause,
  Remove,
  Resume,
  SaveSettings
} from '../wailsjs/go/main/App'

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
}

export type TaskEvent = { kind: 'upsert'; task: Task }
export type PublicConfig = { output: string; maxDownload: string; engineVersion: string }
export type TaskPage = { items: Task[]; total: number }

export const backend = {
  add: (source: string, output: string) => Add(source, output) as Promise<Task>,
  config: () => Config() as Promise<PublicConfig>,
  list: (offset = 0, limit = 100) => ListTasks(offset, limit) as Promise<TaskPage>,
  saveSettings: (output: string, maxDownload: string) => SaveSettings(output, maxDownload),
  openOutput: () => OpenOutput(),
  chooseTorrent: () => ChooseTorrent(),
  chooseOutput: () => ChooseOutput(),
  pause: (id: string) => Pause(id),
  resume: (id: string) => Resume(id),
  remove: (id: string) => Remove(id),
  onTask: (handler: (event: TaskEvent) => void) => {
    EventsOn('task:update', handler)
    return () => EventsOff('task:update')
  }
}
