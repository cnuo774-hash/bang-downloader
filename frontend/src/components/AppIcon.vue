<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number }>(), { size: 20 })
const paths: Record<string, string[]> = {
  download: [
    'M12 3v12m-4-4 4 4 4-4',
    'M4 16v4a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-4'
  ],
  settings: ['M4 7h16M4 17h16', 'M8 4v6M16 14v6'],
  folder: [
    'M3 7V5a1 1 0 0 1 1-1h5l2 3h9a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z'
  ],
  plus: ['M12 5v14M5 12h14'],
  check: ['m5 12 4 4L19 6'],
  close: ['m6 6 12 12M6 18 18 6'],
  play: ['m9 5 10 7-10 7Z'],
  pause: ['M8 5v14M16 5v14'],
  arrow: ['M5 12h14m-5-5 5 5-5 5'],
  file: ['M14 3H5v18h14V8Z', 'M14 3v5h5M8 13h8M8 17h5'],
  link: [
    'm10 13 4-4',
    'M8 15 6 17a4 4 0 0 1-6-6l4-4a4 4 0 0 1 6 0M16 9l2-2a4 4 0 0 1 6 6l-4 4a4 4 0 0 1-6 0'
  ],
  music: [
    'M9 18V5l11-2v13',
    'M9 5v4l11-2',
    'M9 18c0 2-6 3-6 0s6-3 6 0ZM20 16c0 2-6 3-6 0s6-3 6 0Z'
  ],
  activity: ['M3 12h4l3-8 4 16 3-8h4'],
  info: ['M12 11v6M12 7h.01', 'M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0']
}
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path
      v-for="(path, index) in paths[name] || paths.file"
      :key="index"
      :d="path"
    />
  </svg>
</template>
