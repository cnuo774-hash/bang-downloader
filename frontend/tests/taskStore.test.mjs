import test from 'node:test'
import assert from 'node:assert/strict'
import { TaskStore } from '../.test-dist/taskStore.js'

const task = (id, addedAt = 1) => ({ id, addedAt, status: 'active', name: id, completed: 0, total: 100, downloadBps: 0, output: '/downloads', updatedAt: 1 })
const event = (value, revision, total, added = false, kind = 'upsert') => ({ task: value, revision, total, added, kind })

test('updates to unloaded tasks do not advance the pagination offset', () => {
  const store = new TaskStore()
  store.page({ items: [task('new', 3)], total: 3, revision: 1 })
  store.apply(event(task('old', 1), 2, 3))
  assert.equal(store.tasks.length, 1)
  assert.equal(store.total, 3)
  assert.equal(store.page({ items: [task('middle', 2), task('old', 1)], total: 3, revision: 2 }, true), true)
  assert.deepEqual(store.tasks.map(t => t.id), ['new', 'middle', 'old'])
})

test('subscription before initial snapshot neither loses nor duplicates tasks', () => {
  const store = new TaskStore()
  store.apply(event(task('one', 1), 1, 1, true))
  store.apply(event(task('two', 2), 2, 2, true))
  store.page({ items: [task('one', 1)], total: 1, revision: 1 })
  assert.deepEqual(store.tasks.map(t => t.id), ['two', 'one'])
  assert.equal(store.total, 2)
})

test('deletion events remove rows and stale events cannot resurrect them', () => {
  const store = new TaskStore()
  store.page({ items: [task('one')], total: 1, revision: 1 })
  store.apply(event(task('one'), 2, 0, false, 'remove'))
  store.apply(event(task('one'), 1, 1))
  assert.equal(store.tasks.length, 0)
  assert.equal(store.total, 0)
})

test('pages crossing a mutation are rejected for a fresh snapshot', () => {
  const store = new TaskStore()
  store.page({ items: [task('one')], total: 2, revision: 1 })
  store.apply(event(task('new', 2), 2, 3, true))
  assert.equal(store.page({ items: [task('two')], total: 2, revision: 1 }, true), false)
  assert.equal(store.page({ items: [task('two')], total: 3, revision: 3 }, true), false)
  assert.equal(store.tasks.length, 2)
})


test('global summary includes tasks outside the loaded page', () => {
  const store = new TaskStore()
  store.page({ items: [task('one')], total: 250, revision: 1, stats: { active: 150, complete: 100, downloadBps: 2048 } })
  assert.equal(store.stats.active, 150)
  store.apply({ ...event(task('unloaded'), 2, 250), stats: { active: 149, complete: 101, downloadBps: 1024 } })
  assert.equal(store.tasks.length, 1)
  assert.equal(store.stats.complete, 101)
  assert.equal(store.stats.downloadBps, 1024)
})
