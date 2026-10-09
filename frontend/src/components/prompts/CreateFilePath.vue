<template>
  <div>
    <div class="path-container" ref="container">
      <template v-for="(item, index) in path" :key="index">
        /
        <span class="path-item">
          <span
            v-if="isDir === true || index < path.length - 1"
            class="material-icons"
            >folder
          </span>
          <span v-else class="material-icons">insert_drive_file</span>
          {{ item }}
        </span>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { useRoute } from "vue-router";
import { useFileStore } from "@/stores/file";
import url from "@/utils/url";

const fileStore = useFileStore();
const route = useRoute();

const props = defineProps({
  name: {
    type: String,
    required: true,
  },
  isDir: {
    type: Boolean,
    default: false,
  },
  path: {
    type: String,
    default: null,
  },
});

const container = ref<HTMLElement | null>(null);

// The route's path is percent-encoded; its folders read as they are named ("albüm", not
// "alb%C3%BCm"; Gezgin). The name being typed is shown as it is.
const decode = (segment: string) => {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
};

const path = computed(() => {
  const routePath = props.path || route.path;
  const basePath = fileStore.isFiles ? routePath : url.removeLastDir(routePath);
  const folders = basePath.split("/").filter(Boolean).splice(1).map(decode);
  return props.name ? [...folders, props.name] : folders;
});

watch(path, () => {
  nextTick(() => {
    const lastItem = container.value?.lastElementChild;
    lastItem?.scrollIntoView({ behavior: "auto", inline: "end" });
  });
});
</script>

<style scoped>
.path-container {
  display: flex;
  align-items: center;
  margin: 0.2em 0;
  gap: 0.25em;
  overflow-x: auto;
  max-width: 100%;
  scrollbar-width: none;
  opacity: 0.5;
}

.path-container::-webkit-scrollbar {
  display: none;
}

.path-item {
  display: flex;
  align-items: center;
  margin: 0.2em 0;
  gap: 0.25em;
  white-space: nowrap;
}

.path-item > span {
  font-size: 0.9em;
}
</style>
