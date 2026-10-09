<template>
  <div>
    <header-bar showMenu showLogo>
      <title />
    </header-bar>

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
    <h2 class="message" v-else-if="links.length === 0">
      <i class="material-icons">share</i>
      <span>{{ t("shares.nothing") }}</span>
    </h2>
    <div v-else id="shares" class="card file-icons">
      <div class="card-content full">
        <table>
          <thead>
            <tr>
              <th>
                <button
                  class="sort"
                  :class="{ active: sortBy.by === 'name' }"
                  @click="sortOn('name')"
                  :title="t('shares.sortByName')"
                  :aria-label="t('shares.sortByName')"
                >
                  <span>{{ t("shares.name") }}</span>
                  <i class="material-icons">{{ sortIcon("name") }}</i>
                </button>
                <button
                  class="sort end-sort-inline"
                  :class="{ active: sortBy.by === 'end' }"
                  @click="sortOn('end')"
                  :title="t('shares.sortByEnd')"
                  :aria-label="t('shares.sortByEnd')"
                >
                  <span>{{ t("shares.end") }}</span>
                  <i class="material-icons">{{ sortIcon("end") }}</i>
                </button>
              </th>
              <th class="kind">{{ t("shares.kind") }}</th>
              <th class="end">
                <button
                  class="sort"
                  :class="{ active: sortBy.by === 'end' }"
                  @click="sortOn('end')"
                  :title="t('shares.sortByEnd')"
                  :aria-label="t('shares.sortByEnd')"
                >
                  <span>{{ t("shares.end") }}</span>
                  <i class="material-icons">{{ sortIcon("end") }}</i>
                </button>
              </th>
              <th v-if="isAdmin" class="owner">{{ t("shares.owner") }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="link in sorted" :key="link.hash">
              <td>
                <div class="item">
                  <span
                    class="icon"
                    :data-dir="link.isDir"
                    :data-type="link.type"
                    :data-ext="extension(link)"
                    ><i class="material-icons"></i
                  ></span>
                  <div>
                    <p class="name">
                      <router-link v-if="link.open" :to="openURL(link)">{{
                        nameOf(link)
                      }}</router-link>
                      <span v-else>{{ nameOf(link) }}</span>
                      <i
                        v-if="link.hasPassword"
                        class="material-icons lock"
                        :title="t('shares.protected')"
                        :aria-label="t('shares.protected')"
                        >lock</i
                      >
                    </p>
                    <p v-if="folderOf(link)" class="folder">
                      {{ folderOf(link) }}
                    </p>
                    <p v-if="isAdmin" class="folder owner-inline">
                      {{ t("shares.owner") }}:
                      {{ link.owner ?? authStore.user?.username }}
                    </p>
                    <p :class="['end', 'end-inline', endOf(link)]">
                      {{ t("shares.end") }}: {{ endText(link) }}
                    </p>
                    <p class="kind-inline">
                      <span class="badge" :class="link.kind ?? 'link'">{{
                        kindOf(link)
                      }}</span>
                      <span v-if="link.kind === 'webdav'" class="folder">
                        {{ t("shares.webdavUser", { user: link.webdavUser }) }}
                      </span>
                    </p>
                  </div>
                </div>
              </td>
              <td class="kind">
                <span class="badge" :class="link.kind ?? 'link'">{{
                  kindOf(link)
                }}</span>
                <p v-if="link.kind === 'webdav'" class="folder">
                  {{ t("shares.webdavUser", { user: link.webdavUser }) }}
                </p>
              </td>
              <td :class="['end', endOf(link)]" :title="endTitle(link)">
                {{ endText(link) }}
              </td>
              <td v-if="isAdmin" class="owner">
                {{ link.owner ?? authStore.user?.username }}
              </td>
              <td class="actions">
                <button
                  class="action"
                  @click="copyLink(link)"
                  :aria-label="t('buttons.copyToClipboard')"
                  :title="t('buttons.copyToClipboard')"
                >
                  <i class="material-icons">content_paste</i>
                </button>
                <button
                  class="action"
                  @click="edit(link)"
                  :aria-label="t('shares.edit')"
                  :title="t('shares.edit')"
                >
                  <i class="material-icons">edit</i>
                </button>
                <button
                  class="action"
                  @click="remove(link)"
                  :aria-label="t('shares.remove')"
                  :title="t('shares.remove')"
                >
                  <i class="material-icons">delete</i>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { share as api } from "@/api";
import type { SharedItem } from "@/api/share";
import { StatusError } from "@/api/utils";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";
import { shareEnd, sortShares, type ShareSort } from "@/utils/shares";
import { encodePath } from "@/utils/url";
import Errors from "@/views/Errors.vue";
import dayjs from "dayjs";
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

// The "Paylaşılanlar" page (Gezgin, K94-K97): the user's shares, every user's for an admin, in a
// fixed list: what is shared and where it lies, how, until when and by whom; each one's address
// copied, its settings changed in place, or the share removed.

const { t, locale } = useI18n();
const authStore = useAuthStore();
const layoutStore = useLayoutStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const loading = ref<boolean>(true);
const error = ref<StatusError | null>(null);
const links = ref<SharedItem[]>([]);
const sortBy = ref<ShareSort>({ by: "name", asc: true });
const now = ref<number>(Date.now() / 1000);

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

const extension = (link: SharedItem) => {
  const name = link.name ?? "";
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot).toLowerCase() : "";
};

const openURL = (link: SharedItem) =>
  "/files" + encodePath(link.open ?? "/") + (link.isDir ? "/" : "");

const kindOf = (link: SharedItem) => {
  if (link.kind !== "webdav") return t("shares.link");
  return t("shares.webdav", {
    access: link.writable
      ? t("prompts.webdavReadWrite")
      : t("prompts.webdavReadOnly"),
  });
};

const endOf = (link: SharedItem) => shareEnd(link.expire, now.value);

const endText = (link: SharedItem) => {
  switch (endOf(link)) {
    case "permanent":
      return t("shares.permanent");
    case "ended":
      return t("shares.ended");
    default:
      return dayjs(link.expire * 1000).fromNow();
  }
};

const endTitle = (link: SharedItem) =>
  link.expire ? dayjs(link.expire * 1000).format("LLL") : "";

const collator = computed(
  () => new Intl.Collator(locale.value, { numeric: true, sensitivity: "base" })
);

const sorted = computed(() =>
  sortShares(links.value, sortBy.value, nameOf, collator.value)
);

const sortIcon = (by: ShareSort["by"]) =>
  sortBy.value.by === by && !sortBy.value.asc
    ? "arrow_downward"
    : "arrow_upward";

const sortOn = (by: ShareSort["by"]) => {
  sortBy.value =
    sortBy.value.by === by ? { by, asc: !sortBy.value.asc } : { by, asc: true };
};

const load = async () => {
  try {
    links.value = await api.list();
    now.value = Date.now() / 1000;
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
    else $showError(e as Error);
  } finally {
    loading.value = false;
  }
};

const copyLink = (link: SharedItem) => {
  const text = api.getShareURL(link);
  copy({ text }).then(
    () => $showSuccess(t("success.linkCopied")),
    () =>
      copy({ text }, { permission: true }).then(
        () => $showSuccess(t("success.linkCopied")),
        (e) => $showError(e)
      )
  );
};

const edit = (link: SharedItem) => {
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

const remove = (link: SharedItem) => {
  layoutStore.showHover({
    prompt: "share-delete",
    props: { name: nameOf(link) },
    confirm: async () => {
      layoutStore.closeHovers();
      try {
        await api.remove(link.hash);
        links.value = links.value.filter((l) => l.hash !== link.hash);
        $showSuccess(t("shares.removed", { name: nameOf(link) }));
      } catch (e) {
        $showError(e as Error);
      }
    },
  });
};

onMounted(load);
</script>

<style scoped>
#shares {
  margin: 1em 0;
}

#shares .card-content.full {
  padding-top: 0.5em;
}

#shares th button.sort {
  display: inline-flex;
  align-items: center;
  gap: 0.2em;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

#shares th button.sort i {
  font-size: 1em;
  opacity: 0;
}

#shares th button.sort.active i,
#shares th button.sort:hover i {
  opacity: 1;
}

#shares td {
  vertical-align: middle;
}

#shares .item {
  display: flex;
  align-items: center;
  gap: 0.8em;
  min-width: 0;
}

#shares .item .icon i {
  font-size: 2em;
  vertical-align: middle;
}

#shares .item p {
  margin: 0;
}

#shares .name {
  display: flex;
  align-items: center;
  gap: 0.3em;
  color: var(--textSecondary);
  word-break: break-word;
}

#shares .name a:hover {
  text-decoration: underline;
}

#shares .name .lock {
  font-size: 1em;
  color: var(--textPrimary);
}

#shares .folder {
  margin: 0.2em 0 0;
  font-size: 0.85em;
  color: var(--textPrimary);
  word-break: break-word;
}

#shares .badge {
  display: inline-block;
  padding: 0.15em 0.6em;
  border-radius: 1em;
  font-size: 0.85em;
  white-space: nowrap;
  background: var(--surfaceSecondary);
  color: var(--textSecondary);
}

#shares .badge.webdav {
  background: var(--icon-violet);
  color: #fff;
}

#shares td.end {
  white-space: nowrap;
}

#shares .end.soon {
  color: #b7791f;
  font-weight: 500;
}

#shares .end.ended {
  color: var(--red);
}

#shares td.actions {
  white-space: nowrap;
  text-align: right;
}

#shares td.actions .action i {
  padding: 0.25em;
}

#shares .kind-inline .folder {
  margin-left: 0.4em;
}

#shares .kind-inline,
#shares .owner-inline,
#shares .end-inline,
#shares th button.end-sort-inline {
  display: none;
}

/* Narrower, the kind and the owner go under the name. */
@media (max-width: 1024px) {
  #shares th.kind,
  #shares td.kind,
  #shares th.owner,
  #shares td.owner {
    display: none;
  }

  #shares .owner-inline {
    display: block;
  }

  #shares .kind-inline {
    display: block;
    margin-top: 0.3em;
  }
}

/* On a phone, the end too, its sort beside the name's. */
@media (max-width: 736px) {
  #shares th.end,
  #shares td.end {
    display: none;
  }

  #shares .end-inline {
    display: block;
    margin-top: 0.2em;
    font-size: 0.85em;
  }

  #shares th button.end-sort-inline {
    display: inline-flex;
    margin-left: 1em;
  }

  #shares td.actions .action i {
    padding: 0.1em;
  }
}
</style>
