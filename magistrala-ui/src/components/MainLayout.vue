<template>
  <div class="app-layout">
    <Sidebar />
    <main class="main-content"><router-view :key="$route.params.id" /></main>
  </div>
</template>
<script setup>
import Sidebar from './Sidebar.vue'
</script>
