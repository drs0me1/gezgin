<template>
  <div>
    <header-bar showMenu showLogo showNav>
      <title />
    </header-bar>

    <div v-if="isMobile && fileStore.selectedCount > 0" id="file-selection">
      <span>
        {{ t("prompts.selectedCount", { count: fileStore.selectedCount }) }}
      </span>
      <template v-if="selectedShare">
        <action
          icon="content_copy"
          :label="copyLabel(selectedShare)"
          @action="copyLink"
        />
        <action icon="settings" :label="t('shares.edit')" @action="edit" />
      </template>
      <action
        icon="share_off"
        :label="
          fileStore.selectedCount > 1
            ? t('shares.removeMany', { count: fileStore.selectedCount })
            : t('shares.remove')
        "
        @action="remove"
      />
      <action
        v-if="selectedShare"
        icon="info"
        :label="t('shares.info')"
        @action="info"
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
      <i class="material-icons">share</i>
      <span>{{ t("shares.nothing") }}</span>
    </h2>
    <div
      v-else
      id="listing"
      class="file-icons"
      data-clear-on-click="true"
      :class="[
        authStore.user?.viewMode ?? '',
        'shares',
        { 'with-owner': isAdmin },
      ]"
      @click="handleEmptyAreaClick"
      @contextmenu="showContextMenu"
    >
      <!-- A top line: the count, the view and "Seç" (Gezgin, K130). -->
      <div class="listing-bar" data-clear-on-click="true">
        <span class="small">
          {{ t("shares.count", { count: items.length }, items.length) }}
        </span>
        <listing-tools />
      </div>
      <div>
        <div class="item header">
          <div>
            <p
              :class="{ active: sortBy.by === 'name' }"
              class="name"
              role="button"
              tabindex="0"
              @click="sortOn('name')"
              :title="t('shares.sortByName')"
              :aria-label="t('shares.sortByName')"
            >
              <span>{{ t("shares.name") }}</span>
              <i class="material-icons">{{ sortIcon("name") }}</i>
            </p>
            <p class="size">
              <span>{{ t("shares.kind") }}</span>
            </p>
            <p
              :class="{ active: sortBy.by === 'end' }"
              class="modified"
              role="button"
              tabindex="0"
              @click="sortOn('end')"
              :title="t('shares.sortByEnd')"
              :aria-label="t('shares.sortByEnd')"
            >
              <span>{{ t("shares.end") }}</span>
              <i class="material-icons">{{ sortIcon("end") }}</i>
            </p>
            <p v-if="isAdmin" class="owner">
              <span>{{ t("shares.owner") }}</span>
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
            :path="item.path"
            :locked="shareOf(item)?.hasPassword"
            :meta="metaOf(item)"
            :title="
              item.location
                ? `${t('favorites.location')}: ${item.location}`
                : ''
            "
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
        <template v-if="selectedShare">
          <action
            icon="content_copy"
            :label="copyLabel(selectedShare)"
            @action="copyLink"
          />
          <action
            v-if="selectedShare.kind !== 'webdav'"
            icon="open_in_new"
            :label="t('shares.openNewTab')"
            @action="openNewTab"
          />
          <action icon="settings" :label="t('shares.edit')" @action="edit" />
          <action
            v-if="selectedShare.open"
            icon="folder_open"
            :label="t('search.openLocation')"
            @action="openLocation"
          />
          <div class="separator"></div>
        </template>
        <action
          class="danger"
          icon="share_off"
          :label="
            fileStore.selectedCount > 1
              ? t('shares.removeMany', { count: fileStore.selectedCount })
              : t('shares.remove')
          "
          @action="remove"
        />
        <action
          v-if="selectedShare"
          icon="info"
          :label="t('shares.info')"
          @action="info"
        />
      </context-menu>
    </div>
  </div>
</template>

<script setup lang="ts">
import { share as api } from "@/api";
import type { SharedItem } from "@/api/share";
import { StatusError } from "@/api/utils";
import ContextMenu from "@/components/ContextMenu.vue";
import Item from "@/components/files/ListingItem.vue";
import ListingTools from "@/components/files/ListingTools.vue";
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { useAuthStore } from "@/stores/auth";
import { fitColumns } from "@/utils/columns";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { shareEnd, sortShares, type ShareSort } from "@/utils/shares";
import { encodePath } from "@/utils/url";
import Errors from "@/views/Errors.vue";
import dayjs from "dayjs";
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

// The "Paylaşılanlar" page (Gezgin, K94-K98, K130-K134): the user's shares, every user's for an
// admin, in the same view as a folder of files, one tile per share: its item, a lock for a
// password, how it is shared, until when and, for an admin, by whom. A double click opens the
// item in its place; the right-click menu copies the address, opens a link, changes the share,
// opens the item's folder, removes the share or tells about it.

const { t, locale } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const loading = ref<boolean>(true);
const error = ref<StatusError | null>(null);
const links = ref<SharedItem[]>([]);
const sortBy = ref<ShareSort>({ by: "name", asc: true });
const now = ref<number>(Date.now() / 1000);
const width = ref<number>(window.innerWidth);
const isContextMenuVisible = ref<boolean>(false);
const contextMenuPos = ref<{ x: number; y: number }>({ x: 0, y: 0 });

const isMobile = computed(() => width.value <= 736);
const isAdmin = computed(() => !!authStore.user?.perm.admin);

// A share of a whole scope has no name of its own: it is "Dosyalarım", or the folder the admin
// sees it as.
const nameOf = (link: SharedItem) => {
  if (link.name) return link.name;
  if (link.open && link.open !== "/") {
    return link.open.slice(link.open.lastIndexOf("/") + 1);
  }
  return t("sidebar.myFiles");
};

// The folder the item lies in, "Dosyalarım" for the top; none for a whole scope.
const folderOf = (link: SharedItem) => {
  if ((link.open ?? link.path) === "/") return "";
  return link.folder === "/" ? t("sidebar.myFiles") : link.folder;
};

const kindOf = (link: SharedItem) => {
  if (link.kind !== "webdav") return t("shares.link");
  return t("shares.webdav", {
    access: link.writable
      ? t("prompts.webdavReadWrite")
      : t("prompts.webdavReadOnly"),
  });
};

const endText = (link: SharedItem) => {
  switch (shareEnd(link.expire, now.value)) {
    case "permanent":
      return t("shares.permanent");
    case "ended":
      return t("shares.ended");
    default:
      return dayjs(link.expire * 1000).fromNow();
  }
};

const ownerOf = (link: SharedItem) =>
  link.owner ?? authStore.user?.username ?? "";

const collator = computed(
  () => new Intl.Collator(locale.value, { numeric: true, sensitivity: "base" })
);

// The page's listing, one item per share, which the selection and the prompts read as they read
// a folder's; a share is found by its item's id, the share's hash.
const shares = computed(
  () => new Map(links.value.map((link) => [link.hash, link]))
);
const shareOf = (item: ResourceItem) =>
  item.id ? shares.value.get(item.id) : undefined;

const show = () => {
  const ordered = sortShares(links.value, sortBy.value, nameOf, collator.value);
  fileStore.updateRequest({
    path: "/",
    name: t("shares.title"),
    size: 0,
    extension: "",
    modified: "",
    mode: 0,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: "/shares",
    shares: true,
    index: 0,
    numDirs: ordered.filter((l) => l.isDir).length,
    numFiles: ordered.filter((l) => !l.isDir).length,
    sorting: { by: sortBy.value.by, asc: sortBy.value.asc },
    items: ordered.map((link, index) => ({
      index,
      id: link.hash,
      name: nameOf(link),
      path: link.open ?? "",
      url: link.open
        ? "/files" + encodePath(link.open) + (link.isDir ? "/" : "")
        : "",
      isDir: link.isDir,
      size: 0,
      modified: "",
      type: (link.type ?? "") as ResourceType,
      extension: "",
      mode: 0,
      isSymlink: false,
      location: folderOf(link),
    })),
  });
};

const items = computed(() =>
  fileStore.req?.shares ? fileStore.req.items : []
);
const groups = computed(() => [
  { title: "files.folders", items: items.value.filter((i) => i.isDir) },
  { title: "files.files", items: items.value.filter((i) => !i.isDir) },
]);

// A tile's lines: the kind, the end and the owner.
const metaOf = (item: ResourceItem) => {
  const link = shareOf(item);
  if (!link) return undefined;
  return {
    kind: kindOf(link),
    end: endText(link),
    endState: shareEnd(link.expire, now.value),
    owner: isAdmin.value ? ownerOf(link) : undefined,
    foreign: link.owner !== undefined,
  };
};

const selectedShares = computed(() =>
  fileStore.selected
    .map((i) => items.value[i])
    .map((item) => (item ? shareOf(item) : undefined))
    .filter((link): link is SharedItem => link !== undefined)
);
const selectedShare = computed(() =>
  selectedShares.value.length === 1 ? selectedShares.value[0] : undefined
);

const sortIcon = (by: ShareSort["by"]) =>
  sortBy.value.by === by && !sortBy.value.asc
    ? "arrow_downward"
    : "arrow_upward";

const sortOn = (by: ShareSort["by"]) => {
  sortBy.value =
    sortBy.value.by === by ? { by, asc: !sortBy.value.asc } : { by, asc: true };
  show();
};

const load = async () => {
  try {
    links.value = await api.list();
    now.value = Date.now() / 1000;
    show();
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
    else $showError(e as Error);
  } finally {
    loading.value = false;
  }
};

const urlOf = (link: SharedItem) => api.getShareURL(link);

const copyLabel = (link: SharedItem) =>
  link.kind === "webdav" ? t("shares.copyAddress") : t("shares.copyLink");

const copyLink = () => {
  const link = selectedShare.value;
  if (!link) return;
  isContextMenuVisible.value = false;
  const text = urlOf(link);
  copy({ text }).then(
    () => $showSuccess(t("success.linkCopied")),
    () =>
      copy({ text }, { permission: true }).then(
        () => $showSuccess(t("success.linkCopied")),
        (e) => $showError(e)
      )
  );
};

const openNewTab = () => {
  const link = selectedShare.value;
  if (!link) return;
  isContextMenuVisible.value = false;
  window.open(urlOf(link), "_blank", "noopener,noreferrer");
};

const edit = () => {
  const link = selectedShare.value;
  if (!link) return;
  isContextMenuVisible.value = false;
  layoutStore.showHover({
    prompt: "share-edit",
    props: { link, name: nameOf(link) },
    confirm: (changed: Share) => {
      layoutStore.closeHovers();
      links.value = links.value.map((l) =>
        l.hash === changed.hash ? { ...l, ...changed } : l
      );
      now.value = Date.now() / 1000;
      $showSuccess(t("shares.updated", { name: nameOf(link) }));
    },
  });
};

// The folder the item lies in, with the item selected there.
const openLocation = () => {
  const link = selectedShare.value;
  if (!link?.open) return;
  isContextMenuVisible.value = false;
  const parent = link.open.slice(0, link.open.lastIndexOf("/") + 1);
  fileStore.preselect = link.open;
  router.push("/files" + encodePath(parent));
};

// One share after its question, or several after one question for them all.
const remove = () => {
  const chosen = selectedShares.value;
  if (chosen.length === 0) return;
  isContextMenuVisible.value = false;
  const drop = async () => {
    layoutStore.closeHovers();
    let removed = 0;
    for (const link of chosen) {
      try {
        await api.remove(link.hash);
        removed++;
      } catch (e) {
        $showError(e as Error);
      }
    }
    const gone = new Set(chosen.map((l) => l.hash));
    links.value = links.value.filter((l) => !gone.has(l.hash));
    fileStore.selected = [];
    show();
    if (removed === 1 && chosen.length === 1) {
      $showSuccess(t("shares.removed", { name: nameOf(chosen[0]) }));
    } else if (removed > 0) {
      $showSuccess(t("shares.removedMany", { count: removed }));
    }
  };
  if (chosen.length === 1) {
    layoutStore.showHover({
      prompt: "share-delete",
      props: { name: nameOf(chosen[0]) },
      confirm: drop,
    });
    return;
  }
  layoutStore.showHover({
    prompt: "confirm",
    props: {
      message: t("shares.removeManyConfirm", { count: chosen.length }),
      confirm: t("shares.removeMany", { count: chosen.length }),
      danger: true,
    },
    confirm: drop,
  });
};

const info = () => {
  const link = selectedShare.value;
  if (!link) return;
  isContextMenuVisible.value = false;
  layoutStore.showHover({
    prompt: "share-info",
    props: {
      share: link,
      name: nameOf(link),
      folder: folderOf(link),
      kind: kindOf(link),
      url: urlOf(link),
      end: endText(link),
      endDate: link.expire ? dayjs(link.expire * 1000).format("LLL") : "",
      owner: isAdmin.value ? ownerOf(link) : "",
    },
  });
};

// The menu is a share's; off the shares nothing is selected and there is none.
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
  min-height: calc(100vh - 8rem);
}
</style>
