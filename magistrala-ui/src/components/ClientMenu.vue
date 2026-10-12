<!-- filepath: /home/liar/Magistrala/magistrala-ui/src/components/ClientMenu.vue -->
<template>
  <div class="client-menu">
    <div class="client-title">
      <div class="client-name">{{ name }}</div>
      <div class="client-id">{{ id }}</div>
    </div>
    <ul class="menu-list">
      <li
        v-for="item in menu"
        :key="item.key"
        :class="['menu-item', { active: active === item.key }]"
        @click="$emit('menu-click', item.key)"
      >
        {{ item.label }}
      </li>
    </ul>
  </div>
</template>

<script setup>
defineProps({
  name: String,
  id: String,
  menu: Array,
  active: String
})
</script>

<style scoped>
.client-menu {
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 0 24px #eee;
  padding: 24px 0 24px 0;
  min-height: 100%;
}
.client-title {
  padding: 0 24px 16px 24px;
  border-bottom: 1px solid #eee;
}
.client-name {
  font-size: 1.2em;
  font-weight: bold;
}
.client-id {
  font-size: 0.9em;
  color: #888;
  margin-top: 4px;
}
.menu-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.menu-item {
  padding: 16px 24px;
  cursor: pointer;
  color: #174e8a;
  font-weight: 500;
  border-left: 4px solid transparent;
  transition: background 0.2s, border-color 0.2s;
}
.menu-item.active {
  background: #eaf7f6;
  border-left: 4px solid #174e8a;
  color: #174e8a;
  font-weight: bold;
}
.menu-item:hover {
  background: #f4f8fb;
}
</style>