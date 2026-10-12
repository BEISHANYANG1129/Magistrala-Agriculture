<template>
  <div class="channel-menu-vertical">
    <div class="channel-title-row">
      <span class="channel-title">{{ name }}</span>
    </div>
    <div class="channel-menu-list">
      <button
        v-for="item in menu"
        :key="item.key"
        :class="['menu-btn', { active: item.key === active }]"
        @click="$emit('menu-click', item.key)"
      >
        <span v-if="item.icon" class="menu-icon">{{ item.icon }}</span>
        {{ item.label }}
      </button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  name: String,
  id: String,
  menu: Array,
  active: String
})
defineEmits(['menu-click'])
</script>

<style scoped>
.channel-menu-vertical {
  width: 90%;
}
.channel-title-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 32px;
}
.channel-title {
  font-size: 1.35em;
  font-weight: bold;
  color: #0a3566;
}
.channel-menu-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}
.menu-btn {
  background: #eaf7f6;
  border: none;
  border-radius: 12px;
  padding: 14px 0;
  font-size: 1em;
  cursor: pointer;
  font-weight: bold;
  color: #174e8a;
  width: 100%;
  text-align: left;
  padding-left: 28px;
  box-sizing: border-box;
  transition: background 0.2s, color 0.2s;
}
.menu-btn.active {
  background: #174e8a;
  color: #fff;
}
.menu-icon {
  margin-right: 8px;
}
</style>