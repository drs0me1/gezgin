<template>
  <select
    :value="locale"
    @change="emit('update:locale', ($event.target as HTMLSelectElement).value)"
  >
    <option v-for="(name, code) in names" :key="code" :value="code">
      {{ name }}
    </option>
  </select>
</template>

<script setup lang="ts">
// Gezgin speaks Turkish and English only (K142).
defineProps<{
  locale: string;
}>();

const emit = defineEmits<{
  (e: "update:locale", value: string): void;
}>();

const names = { tr: "Türkçe", en: "English" };
</script>
