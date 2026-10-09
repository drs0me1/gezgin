<template>
  <div>
    <header-bar showMenu showLogo showNav>
      <search />
      <title />
      <action
        class="search-button"
        icon="search"
        :label="t('buttons.search')"
        @action="layoutStore.showHover('search')"
      />

      <template #actions>
        <action
          v-if="!isMobile && headerButtons.favorite"
          :icon="isFavorite ? 'star' : 'star_border'"
          :label="isFavorite ? t('buttons.unfavorite') : t('buttons.favorite')"
          @action="toggleFavorite"
        />
        <action
          v-if="isMobile && headerButtons.download"
          icon="file_download"
          :label="t('buttons.download')"
          @action="download"
        />
        <action
          v-if="isMobile && selectedItem"
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
        v-if="selectedItem"
        icon="folder_open"
        :label="t('search.openLocation')"
        @action="openLocation"
      />
      <action
        v-if="headerButtons.share"
        icon="share"
        :label="t('buttons.share')"
        show="share"
      />
      <action
        v-if="headerButtons.favorite"
        :icon="isFavorite ? 'star' : 'star_border'"
        :label="isFavorite ? t('buttons.unfavorite') : t('buttons.favorite')"
        @action="toggleFavorite"
      />
    </div>

    <p class="search-status" role="status">
      <span>
        {{ t("search.results", { query }) }} ·
        {{ t("search.in", { folder: folderName }) }} ·
        {{ t("search.count", { count: items.length }, items.length) }}
      </span>
      <template v-if="ongoing">
        <span class="searching">{{ t("search.searching") }}</span>
        <button class="button button--flat" @click="stop">
          {{ t("search.stop") }}
        </button>
      </template>
      <span v-else-if="capped" class="capped">
        {{ t("search.capped", { max: MAX_RESULTS }) }}
      </span>
      <listing-tools />
    </p>

    <errors v-if="error" :errorCode="error.status" />
    <h2 class="message" v-else-if="!ongoing && items.length === 0">
      <i class="material-icons">search_off</i>
      <span>{{ t("search.none") }}</span>
    </h2>
    <div
      v-else
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
              :class="[column.by, { active: sorted(column.by) }]"
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
            :key="item.path"
            :index="item.index"
            :name="item.name"
            :isDir="item.isDir"
            :url="item.url"
            :modified="item.modified"
            :type="item.type"
            :size="item.size"
            :path="item.path"
            :favorite="item.favorite"
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
          icon="folder_open"
          :label="t('search.openLocation')"
          @action="openLocation"
        />
        <action
          v-if="headerButtons.share"
          icon="share"
          :label="t('buttons.share')"
          show="share"
        />
        <action
          v-if="headerButtons.favorite"
          :icon="isFavorite ? 'star' : 'star_border'"
          :label="isFavorite ? t('buttons.unfavorite') : t('buttons.favorite')"
          @action="toggleFavorite"
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
import { files as filesApi, search as searchApi, users } from "@/api";
import { StatusError } from "@/api/utils";
import ContextMenu from "@/components/ContextMenu.vue";
import Search from "@/components/Search.vue";
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Item from "@/components/files/ListingItem.vue";
import ListingTools from "@/components/files/ListingTools.vue";
import { useAuthStore } from "@/stores/auth";
import { useFavoritesStore } from "@/stores/favorites";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { orderItems } from "@/utils/listingOrder";
import { encodePath } from "@/utils/url";
import Errors from "@/views/Errors.vue";
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

// The search results (Gezgin, K111): what the search finds under a folder, in the same view as a
// folder of files, each a shortcut that opens its item in its place, coming in as they are found.
// A page of its own, so that back returns to the folder; the query is the search bar's, whose
// conditions (type:image, case:sensitive) choose the kinds of item.

const MAX_RESULTS = 500;

const { t, locale } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const favoritesStore = useFavoritesStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const ongoing = ref<boolean>(false);
const capped = ref<boolean>(false);
const error = ref<StatusError | null>(null);
const width = ref<number>(window.innerWidth);
const isContextMenuVisible = ref<boolean>(false);
const contextMenuPos = ref<{ x: number; y: number }>({ x: 0, y: 0 });
let controller = new AbortController();

const isMobile = computed(() => width.value <= 736);

// The folder searched, as the route has it (encoded, ending in a slash), and the query.
const base = computed(() => {
  const p = route.path.slice("/search".length) || "/";
  return p.endsWith("/") ? p : p + "/";
});
const query = computed(() => String(route.query.q ?? "").trim());
const folderName = computed(() => {
  const folder = decodeURIComponent(base.value).replace(/^\/+|\/+$/g, "");
  return folder === "" ? t("sidebar.myFiles") : folder;
});

const items = computed(() =>
  fileStore.req?.search ? fileStore.req.items : []
);
const collator = computed(
  () => new Intl.Collator(locale.value, { numeric: true, sensitivity: "base" })
);
const groups = computed(() => {
  const ordered = orderItems(
    items.value,
    authStore.user?.sorting,
    collator.value
  );
  return [
    { title: "files.folders", items: ordered.filter((i) => i.isDir) },
    { title: "files.files", items: ordered.filter((i) => !i.isDir) },
  ];
});

const selectedItem = computed(() =>
  fileStore.selectedCount === 1 ? items.value[fileStore.selected[0]] : undefined
);
const isFavorite = computed(() => !!selectedItem.value?.favorite);

const headerButtons = computed(() => ({
  favorite: selectedItem.value !== undefined,
  share:
    selectedItem.value !== undefined &&
    authStore.user?.perm.share &&
    authStore.user?.perm.download,
  download: selectedItem.value !== undefined && authStore.user?.perm.download,
}));

const columns = computed(() => [
  { by: "name", label: t("files.name") },
  { by: "size", label: t("files.size") },
  { by: "modified", label: t("files.lastModified") },
]);

// The page's listing, which the prompts (share, info) read as they read a folder's.
const start = () => {
  fileStore.updateRequest({
    path: decodeURIComponent(base.value),
    name: t("search.title"),
    size: 0,
    extension: "",
    modified: "",
    mode: 0,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: route.fullPath,
    search: true,
    index: 0,
    numDirs: 0,
    numFiles: 0,
    sorting: authStore.user?.sorting ?? { by: "name", asc: false },
    items: [],
  });
};

// A result as a shortcut: its place in the user's files, and where it lies.
const toItem = (found: any, index: number): ResourceItem => {
  const path = decodeURIComponent(base.value) + found.path;
  const parent = path.slice(0, path.lastIndexOf("/"));
  return {
    index,
    name: found.name,
    path,
    url: "/files" + encodePath(path) + (found.isDir ? "/" : ""),
    isDir: found.isDir,
    size: found.size ?? 0,
    modified: found.modified,
    type: (found.type ?? "") as ResourceType,
    extension: "",
    mode: 0,
    isSymlink: false,
    location: parent === "" ? t("sidebar.myFiles") : parent,
    favorite: !!found.favorite,
    sharedLinks: found.sharedLinks,
    sharedDav: found.sharedDav,
  };
};

// The results come in batches, so that a search finding many does not redraw for each.
let batch: any[] = [];
let flushTimer: number | null = null;
const flush = () => {
  flushTimer = null;
  const req = fileStore.req;
  if (!req?.search || batch.length === 0) return;
  for (const found of batch) {
    const item = toItem(found, req.items.length);
    req.items.push(item);
    if (item.isDir) req.numDirs++;
    else req.numFiles++;
  }
  batch = [];
};

const run = async () => {
  controller.abort();
  controller = new AbortController();
  const signal = controller.signal;
  batch = [];
  error.value = null;
  capped.value = false;
  start();
  if (query.value === "") return;

  ongoing.value = true;
  let count = 0;
  try {
    await searchApi(base.value, query.value, signal, (found) => {
      if (count >= MAX_RESULTS) return;
      batch.push(found);
      count++;
      if (count >= MAX_RESULTS) {
        capped.value = true;
        controller.abort();
      }
      if (flushTimer === null) flushTimer = window.setTimeout(flush, 150);
    });
  } catch (e) {
    if (!(e instanceof StatusError && e.is_canceled)) {
      if (e instanceof StatusError && (e.status ?? 0) >= 400) error.value = e;
      else if (!signal.aborted) $showError(e as Error);
    }
  } finally {
    if (signal === controller.signal) {
      if (flushTimer !== null) window.clearTimeout(flushTimer);
      flush();
      ongoing.value = false;
    }
  }
};

const stop = () => {
  controller.abort();
};

// The folder the selected result lies in, with the result selected there.
const openLocation = () => {
  const item = selectedItem.value;
  if (item === undefined) return;
  isContextMenuVisible.value = false;
  const parent = item.path.slice(0, item.path.lastIndexOf("/") + 1);
  fileStore.preselect = item.path;
  router.push("/files" + encodePath(parent));
};

const toggleFavorite = async () => {
  const item = selectedItem.value;
  if (item === undefined) return;
  isContextMenuVisible.value = false;
  try {
    if (item.favorite) {
      await favoritesStore.remove(item.path);
      item.favorite = false;
      $showSuccess(t("favorites.removed", { name: item.name }));
    } else {
      await favoritesStore.add(item.path);
      item.favorite = true;
      $showSuccess(t("favorites.added", { name: item.name }));
    }
  } catch (e: any) {
    $showError(e);
  }
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

const sorted = (by: string) => authStore.user?.sorting?.by === by;

// The arrows and the order follow the folder listing's (FileListing.vue).
const sortIcon = (by: string) => {
  const asc = authStore.user?.sorting?.asc ?? false;
  if (by === "name") {
    return sorted("name") && !asc ? "arrow_upward" : "arrow_downward";
  }
  return sorted(by) && asc ? "arrow_downward" : "arrow_upward";
};

const sort = async (by: string) => {
  const asc = sortIcon(by) === "arrow_upward";
  const data = { id: authStore.user?.id, sorting: { by, asc } };
  authStore.updateUser(data);
  users.update(data, ["sorting"]).catch($showError);
};

// The menu is a result's; off the results nothing is selected and there is none.
const showContextMenu = (event: MouseEvent) => {
  event.preventDefault();
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".item:not(.header)")) {
    fileStore.selected = [];
    isContextMenuVisible.value = false;
    return;
  }
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
};

// A new query or folder searches again; leaving the page does not.
watch(
  () => route.fullPath,
  () => {
    if (route.name === "Search") run();
  }
);

onMounted(() => {
  window.addEventListener("resize", resize);
  run();
});

onBeforeUnmount(() => {
  window.removeEventListener("resize", resize);
  controller.abort();
  if (flushTimer !== null) window.clearTimeout(flushTimer);
  fileStore.updateRequest(null);
});
</script>

<style scoped>
.search-status {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5em 1em;
  margin: 0.5em 0.5em 1em;
  color: var(--textPrimary);
  font-size: 0.95em;
}

.search-status .searching {
  color: var(--blue);
}

.search-status .button {
  padding: 0.3em 0.8em;
}

#listing {
  min-height: calc(100vh - 10rem);
}
</style>
