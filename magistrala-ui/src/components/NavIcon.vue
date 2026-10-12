<template>
  <svg
    class="nav-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] || paths.platform" />
  </svg>
</template>
<script setup>
defineProps({ name: String })
const paths = {
  bell: 'M5 10a7 7 0 0 1 14 0v6l2 2H3l2-2ZM9 21h6',
  'alert-triangle': 'M12 3 2 21h20ZM12 9v5m0 3v1',
  'check-circle': 'M22 11a10 10 0 1 1-6-9M8 11l4 4L22 4',
  clock: 'M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0ZM12 6v6l4 2',
  user: 'M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM4 21v-2a8 8 0 0 1 16 0v2',
  shield: 'M12 3 3 7v5c0 5 9 10 9 10s9-5 9-10V7Z',
  zap: 'M13 2 3 14h8l-1 8 11-12h-8Z',
  activity: 'M2 12h5l3-9 4 18 3-9h5',
  home: 'M3 10 12 3l9 7v11h-6v-7H9v7H3Z',
  dashboards: 'M3 3h7v7H3ZM14 3h7v7h-7ZM3 14h7v7H3ZM14 14h7v7h-7Z',
  info: 'M4 21V3h16v18ZM8 7h2m4 0h2M8 11h2m4 0h2M10 21v-5h4v5',
  users:
    'M15 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM5 21v-3a7 7 0 0 1 14 0v3M19 4a3 3 0 0 1 0 6M22 18v-2a5 5 0 0 0-3-4',
  'role-management': 'm12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6ZM8 12l3 3 5-6',
  invitations: 'M3 5h18v14H3ZM3 5l9 8 9-8',
  groups: 'M4 4h6v6H4Zm10 10h6v6h-6ZM7 10v7h7M14 4h6v6h-6Z',
  devices: 'M6 3h12v18H6ZM10 17h4M9 6h6v7H9Z',
  channels: 'M9 9a4 4 0 0 0 0 6m6-6a4 4 0 0 1 0 6M6 5a9 9 0 0 0 0 14M18 5a9 9 0 0 1 0 14M12 11v2',
  gateways: 'M3 14h18v7H3ZM7 17v1m4-1v1M12 14V9M8 6a6 6 0 0 1 8 0M10 9a3 3 0 0 1 4 0',
  'device-types': 'M8 3h8v4h4v10h-4v4H8v-4H4V7h4ZM8 7h8v10H8Z',
  bootstraps: 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
  messages: 'M3 4h18v13H9l-6 4ZM7 8h10M7 12h6',
  rules: 'M3 3h6v6H3Zm12 12h6v6h-6ZM9 6h9v9M6 9v9h9',
  'custom-nodes': 'm8 6-6 6 6 6m8-12 6 6-6 6M14 3l-4 18',
  alarms: 'M5 10a7 7 0 0 1 14 0v6l2 2H3l2-2ZM9 21h6',
  reports: 'M5 3h10l4 4v14H5ZM14 3v5h5M8 17v-4m4 4v-6m4 6v-2',
  'field-map': 'm3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2ZM9 3v16m6-14v16',
  agent: 'M5 7h14v13H5ZM12 3v4M9 11v2m6-2v2M9 16h6M2 10v6m20-6v6',
  'agri-integration': 'M4 8h16M4 16h16M8 4 4 8l4 4m8 0 4 4-4 4',
  account: 'M15 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM5 21v-3a7 7 0 0 1 14 0v3',
  platform: 'M4 3h16v6H4Zm0 12h16v6H4ZM8 6h1m-1 12h1M6 9v6m12-6v6',
  menu: 'M4 6h16M4 12h16M4 18h16',
  arrow: 'M5 12h14m-5-5 5 5-5 5',
}
</script>
