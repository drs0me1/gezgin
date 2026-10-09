<template>
  <div>
    <header-bar showMenu showLogo showNav>
      <title />

      <template #actions>
        <!-- On a computer restore, delete for good and Info are in the right-click menu, the whole
             trash's too (Gezgin, K109, K121); a phone, without right-click, keeps them here. -->
        <action
          v-if="isMobile"
          icon="info"
          :label="t('buttons.info')"
          show="info"
        />
        <template v-if="isMobile && items.length > 0">
          <action
            v-if="canRestore"
            icon="restore_from_trash"
            :label="t('trash.restoreAll')"
            @action="restoreAll"
          />
          <action
            v-if="canDelete"
            icon="delete_forever"
            :label="t('trash.deleteAll')"
            @action="deleteAll"
          />
        </template>
      </template>
    </header-bar>

    <div v-if="isMobile && fileStore.selectedCount > 0" id="file-selection">
      <span>
        {{ t("prompts.selectedCount", { count: fileStore.selectedCount }) }}
      </span>
      <action
        v-if="canRestore"
        icon="restore_from_trash"
        :label="t('trash.restore')"
        @action="restoreSelected"
      />
      <action
        v-if="canDelete"
        icon="delete_forever"
        :label="t('trash.deletePermanently')"
        @action="purgeSelected"
      />
    </div>

    <div v-if="loading">
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
    <errors v-else-if="error" :errorCode="error.status" />
    <h2 class="message" v-else-if="items.length === 0">
      <i class="material-icons">delete_outline</i>
      <span>{{ t("trash.nothing") }}</span>
    </h2>
    <template v-else>
      <!-- The page's line: the count and size, the view and "Seç" (Gezgin, K137). -->
      <div class="page-bar">
        <span class="page-bar-text">
          {{ t("trash.summary", { count: summary.count }) }} ·
          {{ filesize(summary.size) }}
        </span>
        <listing-tools />
      </div>

      <div
        id="listing"
        class="file-icons"
        data-clear-on-click="true"
        :class="authStore.user?.viewMode ?? ''"
        @click="handleEmptyAreaClick"
        @contextmenu="showContextMenu"
      >
        <div>
          <div class="item header">
            <div>
              <p
                v-for="column in columns"
                :key="column.by"
                :class="[column.by, { active: sorting.by === column.by }]"
                role="button"
                tabindex="0"
                @click="sort(column.by)"
                :title="column.label"
                :aria-label="column.label"
              >
                <span>{{ column.label }}</span>
                <i class="material-icons">{{ sortIcon(column.by) }}</i>
              </p>
            </div>
          </div>
        </div>

        <template v-for="group in groups" :key="group.title">
          <h2 data-clear-on-click="true" v-if="group.items.length > 0">
            {{ t(group.title) }}
          </h2>
          <div v-if="group.items.length > 0" data-clear-on-click="true">
            <item
              v-for="item in group.items"
              :key="item.id"
              :index="item.index"
              :name="item.name"
              :isDir="item.isDir"
              :url="item.url"
              :modified="item.modified"
              :type="item.type"
              :size="item.size"
              :count="item.count"
              :title="`${t('trash.origin')}: ${item.location}`"
              trashed
            >
            </item>
          </div>
        </template>

        <context-menu
          :show="isContextMenuVisible"
          :pos="contextMenuPos"
          @hide="isContextMenuVisible = false"
        >
          <action
            v-if="canRestore && fileStore.selectedCount > 0"
            icon="restore_from_trash"
            :label="t('trash.restore')"
            @action="restoreSelected"
          />
          <action
            v-if="canDelete && fileStore.selectedCount > 0"
            icon="delete_forever"
            :label="t('trash.deletePermanently')"
            @action="purgeSelected"
          />
          <!-- Off the items, the whole trash (Gezgin, K121). -->
          <template v-if="fileStore.selectedCount === 0">
            <action
              v-if="canRestore"
              icon="restore_from_trash"
              :label="t('trash.restoreAll')"
              @action="restoreAll"
            />
            <action
              v-if="canDelete"
              icon="delete_forever"
              :label="t('trash.deleteAll')"
              @action="deleteAll"
            />
            <div v-if="canRestore || canDelete" class="separator"></div>
          </template>
          <action icon="info" :label="t('buttons.info')" show="info" />
        </context-menu>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { trash as api, users } from "@/api";
import { StatusError } from "@/api/utils";
import ContextMenu from "@/components/ContextMenu.vue";
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Item from "@/components/files/ListingItem.vue";
import ListingTools from "@/components/files/ListingTools.vue";
import { useAuthStore } from "@/stores/auth";
import { fitColumns } from "@/utils/columns";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { filesize } from "@/utils";
import Errors from "@/views/Errors.vue";
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The trash (Gezgin): its items in the same view as a folder of the user's files. No path reaches
// them, so they do not open; they are restored or deleted for good, and the trash is emptied.

const { t } = useI18n();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const list = ref<ITrashList>({ items: [], count: 0, size: 0 });
const loading = ref<boolean>(true);
const error = ref<StatusError | null>(null);
const width = ref<number>(window.innerWidth);
const isContextMenuVisible = ref<boolean>(false);
const contextMenuPos = ref<{ x: number; y: number }>({ x: 0, y: 0 });

const isMobile = computed(() => width.value <= 736);
const canRestore = computed(() => authStore.user?.perm.create === true);
const canDelete = computed(() => authStore.user?.perm.delete === true);

const summary = computed(() => list.value);
const items = computed(() => (fileStore.req?.trash ? fileStore.req.items : []));
const groups = computed(() => [
  { title: "files.folders", items: items.value.filter((i) => i.isDir) },
  { title: "files.files", items: items.value.filter((i) => !i.isDir) },
]);

// The user's sorting, as their folders are sorted; "modified" is when an item was deleted.
const sorting = computed(
  () => authStore.user?.sorting ?? { by: "name", asc: false }
);
const columns = computed(() => [
  { by: "name", label: t("files.name") },
  { by: "size", label: t("files.size") },
  { by: "modified", label: t("trash.deleted") },
]);

const selectedIds = () =>
  fileStore.selected
    .map((i) => items.value[i]?.id)
    .filter((id): id is string => id !== undefined);

// Where an item was: its folder, or "Dosyalarım" for the top.
const locationOf = (origin: string) =>
  origin === "" || origin === "/" ? t("sidebar.myFiles") : origin;

// The server's order for a folder (files.Listing.ApplySort): folders first by name, and File
// Browser's reading of "asc".
const ordered = (entries: ITrashItem[]) => {
  const { by, asc } = sorting.value;
  const byName = (a: ITrashItem, b: ITrashItem) =>
    a.name.localeCompare(b.name, undefined, { numeric: true });
  const sortedEntries = [...entries];
  if (by === "size" || by === "modified") {
    const key = (e: ITrashItem) =>
      by === "size" ? e.size : Date.parse(e.deleted);
    sortedEntries.sort((a, b) =>
      a.isDir !== b.isDir ? (a.isDir ? -1 : 1) : key(a) - key(b)
    );
    if (!asc) sortedEntries.reverse();
    return sortedEntries;
  }
  sortedEntries.sort((a, b) =>
    a.isDir !== b.isDir ? (a.isDir ? -1 : 1) : asc ? byName(b, a) : byName(a, b)
  );
  return sortedEntries;
};

const show = () => {
  const entries = ordered(list.value.items);
  fileStore.updateRequest({
    path: "/",
    name: t("trash.title"),
    size: list.value.size,
    extension: "",
    modified: "",
    mode: 0,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: "/trash/",
    trash: true,
    index: 0,
    numDirs: entries.filter((e) => e.isDir).length,
    numFiles: entries.filter((e) => !e.isDir).length,
    sorting: sorting.value,
    items: entries.map((entry, index) => ({
      id: entry.id,
      index,
      name: entry.name,
      path: "",
      url: "",
      size: entry.size,
      count: entry.count,
      extension: entry.isDir ? "" : (entry.name.match(/\.[^.]+$/)?.[0] ?? ""),
      modified: entry.deleted,
      mode: 0,
      isDir: entry.isDir,
      isSymlink: false,
      type: entry.isDir ? ("dir" as ResourceType) : (entry.type ?? "blob"),
      location: locationOf(entry.origin),
    })),
  });
};

const load = async () => {
  try {
    list.value = await api.list();
    show();
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
  } finally {
    loading.value = false;
  }
};

const restoreSelected = async () => {
  isContextMenuVisible.value = false;
  try {
    const done = await api.restore(selectedIds());
    $showSuccess(t("trash.restored", { count: done.length }));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

const purgeSelected = async () => {
  isContextMenuVisible.value = false;
  try {
    await api.purge(selectedIds());
    $showSuccess(t("trash.purged"));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

// The whole trash, after a question (Gezgin, K121): everything back in its place, or gone for good.
const restoreAll = () => {
  isContextMenuVisible.value = false;
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("trash.restoreAllConfirm", { count: summary.value.count }),
      confirm: t("trash.restore"),
    },
    confirm: async () => {
      layoutStore.closeHovers();
      try {
        const done = await api.restore(list.value.items.map((i) => i.id));
        $showSuccess(t("trash.restored", { count: done.length }));
      } catch (e: any) {
        $showError(e);
      }
      await load();
    },
  });
};

const deleteAll = () => {
  isContextMenuVisible.value = false;
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("trash.deleteAllConfirm", { count: summary.value.count }),
      confirm: t("trash.confirmEmpty"),
      danger: true,
    },
    confirm: async () => {
      layoutStore.closeHovers();
      try {
        await api.empty();
        $showSuccess(t("trash.emptied"));
      } catch (e: any) {
        $showError(e);
      }
      await load();
    },
  });
};

// The arrows follow the folder listing's (FileListing.vue).
const sortIcon = (by: string) => {
  const { asc } = sorting.value;
  const active = sorting.value.by === by;
  if (by === "name") return active && !asc ? "arrow_upward" : "arrow_downward";
  return active && asc ? "arrow_downward" : "arrow_upward";
};

const sort = async (by: string) => {
  const asc = sortIcon(by) === "arrow_upward";
  const data = { id: authStore.user?.id, sorting: { by, asc } };
  authStore.updateUser(data);
  show();
  try {
    if (data.id) await users.update(data, ["sorting"]);
  } catch (e: any) {
    $showError(e);
  }
};

const showContextMenu = (event: MouseEvent) => {
  event.preventDefault();
  // Off the items, nothing stays selected, as in a file manager (Gezgin, K107).
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".item:not(.header)")) fileStore.selected = [];
  isContextMenuVisible.value = true;
  contextMenuPos.value = {
    x: event.clientX + 8,
    y: event.clientY + Math.floor(window.scrollY),
  };
};

const handleEmptyAreaClick = (e: MouseEvent) => {
  const target = e.target;
  if (target instanceof HTMLElement && target.dataset.clearOnClick === "true") {
    fileStore.selected = [];
  }
};

const resize = () => {
  width.value = window.innerWidth;
  fitColumns();
};

onMounted(() => {
  window.addEventListener("resize", resize);
  fitColumns();
  load();
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", resize);
  fileStore.updateRequest(null);
});
</script>

<style scoped>
#listing {
  min-height: calc(100vh - 11rem);
}
</style>
