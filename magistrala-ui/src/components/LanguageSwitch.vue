<template>
  <select
    :value="language"
    @change="setLanguage($event.target.value)"
    :aria-label="tr('界面语言', 'Interface language')"
  >
    <option value="zh">简体中文</option>
    <option value="en">English</option>
  </select>
</template>
<script setup>
import { language, setLanguage, tr } from '../i18n'
</script>
