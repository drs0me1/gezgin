<template>
  <div class="setting-row" :class="{ stack }">
    <div class="setting-label">
      <label v-if="labelFor" :for="labelFor">{{ label }}</label>
      <span v-else>{{ label }}</span>
      <p v-if="help" class="setting-help">{{ help }}</p>
    </div>
    <div class="setting-control"><slot /></div>
  </div>
</template>

<script setup lang="ts">
// One setting (Gezgin, K151): its name and a short help at the left, its control at the right;
// "stack" puts a wide control under the name on a phone.
defineProps<{
  label: string;
  help?: string;
  labelFor?: string;
  stack?: boolean;
}>();
</script>
