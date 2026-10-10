<template>
  <div>
    <header-bar showMenu showLogo showNav>
      <title />

      <template #actions>
        <!-- On a computer the right-click menu holds an item's actions; of them the header keeps
             only the star (Gezgin, K109). A phone has no right-click: it keeps its bar and ⋮. -->
        <action
          v-if="!isMobile && headerButtons.favorite"
          icon="star"
          :label="t('buttons.unfavorite')"
          @action="unfavorite"
        />
        <action
          v-if="isMobile && headerButtons.download"
          icon="file_download"
          :label="t('buttons.download')"
          @action="download"
        />
        <action
          v-if="isMobile"
          icon="info"
          :label="t('buttons.info')"
          show="info"
        />
      </template>
    </header-bar>

    <div v-if="isMobile && fileStore.selectedCount > 0" id="file-selection">
      <span>
        {{ t("prompts.selectedCount", { count: fileStore.selectedCount }) }}
      </span>
      <action
        v-if="headerButtons.share"
        icon="share"
        :label="t('buttons.share')"
        show="share"
      />
      <action
        v-if="headerButtons.favorite"
        icon="star"
        :label="t('buttons.unfavorite')"
        @action="unfavorite"
      />
      <action
        v-if="headerButtons.download"
        icon="file_download"
        :label="t('buttons.download')"
        @action="download"
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
    <!-- The page's line: the count, the view and "Seç", fixed as the folder's path is (K137). -->
    <div v-if="!loading && !error" class="page-bar">
      <span class="page-bar-text">
        {{ t("files.itemCount", { count: items.length }, items.length) }}
      </span>
      <listing-tools v-if="items.length > 0" />
    </div>
    <h2 class="message" v-if="!loading && !error && items.length === 0">
      <i class="material-icons">star_border</i>
      <span>{{ t("favorites.nothing") }}</span>
    </h2>
    <div
      v-else-if="!loading && !error"
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
              :class="{ active: sorted('name') }"
              class="name"
              role="button"
              tabindex="0"
              @click="sort('name')"
              :title="t('files.sortByName')"
              :aria-label="t('files.sortByName')"
            >
              <span>{{ t("files.name") }}</span>
              <i class="material-icons">{{ sortIcon("name") }}</i>
            </p>
            <p
              :class="{ active: sorted('size') }"
              class="size"
              role="button"
              tabindex="0"
              @click="sort('size')"
              :title="t('files.sortBySize')"
              :aria-label="t('files.sortBySize')"
            >
              <span>{{ t("files.size") }}</span>
              <i class="material-icons">{{ sortIcon("size") }}</i>
            </p>
            <p
              :class="{ active: sorted('modified') }"
              class="modified"
              role="button"
              tabindex="0"
              @click="sort('modified')"
              :title="t('files.sortByLastModified')"
              :aria-label="t('files.sortByLastModified')"
            >
              <span>{{ t("files.lastModified") }}</span>
              <i class="material-icons">{{ sortIcon("modified") }}</i>
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
            :key="item.path"
            :index="item.index"
            :name="item.name"
            :isDir="item.isDir"
            :url="item.url"
            :modified="item.modified"
            :type="item.type"
            :size="item.size"
            :count="item.count"
            :sizeUnknown="item.sizeUnknown"
            :path="item.path"
            :sharedLinks="item.sharedLinks"
            :sharedDav="item.sharedDav"
            :title="`${t('favorites.location')}: ${item.location}`"
            shortcut
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
          v-if="headerButtons.share"
          icon="share"
          :label="t('buttons.share')"
          show="share"
        />
        <action
          v-if="headerButtons.favorite"
          icon="star"
          :label="t('buttons.unfavorite')"
          @action="unfavorite"
        />
        <action
          v-if="headerButtons.download"
          icon="file_download"
          :label="t('buttons.download')"
          @action="download"
        />
        <action icon="info" :label="t('buttons.info')" show="info" />
      </context-menu>
    </div>
  </div>
</template>

<script setup lang="ts">
import { favorites as favoritesApi, files as filesApi, users } from "@/api";
import { StatusError } from "@/api/utils";
import ContextMenu from "@/components/ContextMenu.vue";
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Item from "@/components/files/ListingItem.vue";
import ListingTools from "@/components/files/ListingTools.vue";
import { useAuthStore } from "@/stores/auth";
import { fitColumns } from "@/utils/columns";
import { useFavoritesStore } from "@/stores/favorites";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { encodePath } from "@/utils/url";
import Errors from "@/views/Errors.vue";
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

// The "Favoriler" page (Gezgin, K90, K91, K123): the favourites in the same view as a folder of
// the user's files, each tile a shortcut that opens its item in its place. Here an item can be
// opened, shared, downloaded, looked at and taken out of the favourites; the rest is done in its
// own folder.

const { t } = useI18n();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const favoritesStore = useFavoritesStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const loading = ref<boolean>(true);
const error = ref<StatusError | null>(null);
const width = ref<number>(window.innerWidth);
const isContextMenuVisible = ref<boolean>(false);
const contextMenuPos = ref<{ x: number; y: number }>({ x: 0, y: 0 });

const isMobile = computed(() => width.value <= 736);

const items = computed(() =>
  fileStore.req?.favorites ? fileStore.req.items : []
);
const groups = computed(() => [
  { title: "files.folders", items: items.value.filter((i) => i.isDir) },
  { title: "files.files", items: items.value.filter((i) => !i.isDir) },
]);

const selectedItem = computed(() =>
  fileStore.selectedCount === 1 ? items.value[fileStore.selected[0]] : undefined
);

const headerButtons = computed(() => ({
  favorite: selectedItem.value !== undefined,
  share:
    selectedItem.value !== undefined &&
    authStore.user?.perm.share &&
    authStore.user?.perm.download,
  download: selectedItem.value !== undefined && authStore.user?.perm.download,
}));

// Where an item lies: its folder, or "Dosyalarım" for the top.
const locationOf = (path: string) => {
  const parent = path.slice(0, path.lastIndexOf("/"));
  return parent === "" ? t("sidebar.myFiles") : parent;
};

const load = async () => {
  try {
    const listing = await favoritesApi.list(true);
    favoritesStore.keep(listing);
    fileStore.updateRequest({
      path: "/",
      name: t("favorites.title"),
      size: 0,
      extension: "",
      modified: "",
      mode: 0,
      isDir: true,
      isSymlink: false,
      type: "dir" as ResourceType,
      url: "/favorites/",
      favorites: true,
      index: 0,
      numDirs: listing.numDirs,
      numFiles: listing.numFiles,
      sorting: listing.sorting,
      items: listing.items.map((item, index) => ({
        ...item,
        index,
        // A shortcut opens its item where it lies.
        url: "/files" + encodePath(item.path) + (item.isDir ? "/" : ""),
        location: locationOf(item.path),
      })),
    });
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
  } finally {
    loading.value = false;
  }
};

const unfavorite = async () => {
  const item = selectedItem.value;
  if (item === undefined) return;
  layoutStore.closeHovers();
  isContextMenuVisible.value = false;
  try {
    await favoritesStore.remove(item.path);
    $showSuccess(t("favorites.removed", { name: item.name }));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

// One item: a file downloads as it is, a folder as an archive of the chosen format.
const download = () => {
  const item = selectedItem.value;
  if (item === undefined) return;
  if (!item.isDir) {
    filesApi.download(null, item.url);
    return;
  }
  layoutStore.showHover({
    prompt: "download",
    confirm: (format: any) => {
      layoutStore.closeHovers();
      filesApi.download(format, item.url);
    },
  });
};

const sorted = (by: string) => fileStore.req?.sorting?.by === by;

// The arrows and the order follow the folder listing's (FileListing.vue).
const sortIcon = (by: string) => {
  const asc = fileStore.req?.sorting?.asc ?? false;
  if (by === "name") {
    return sorted("name") && !asc ? "arrow_upward" : "arrow_downward";
  }
  return sorted(by) && asc ? "arrow_downward" : "arrow_upward";
};

const sort = async (by: string) => {
  const asc = sortIcon(by) === "arrow_upward";
  try {
    if (authStore.user?.id) {
      await users.update({ id: authStore.user.id, sorting: { by, asc } }, [
        "sorting",
      ]);
    }
  } catch (e: any) {
    $showError(e);
  }
  await load();
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

// A share made from here, or a closed prompt, may ask for a reload.
watch(
  () => fileStore.reload,
  (reload) => {
    if (!reload) return;
    fileStore.reload = false;
    load();
  }
);

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
  min-height: calc(100vh - 8rem);
}
</style>
