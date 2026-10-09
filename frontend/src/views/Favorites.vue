<template>
  <div class="dashboard">
    <header-bar showMenu showLogo />

    <errors v-if="error" :errorCode="error.status" />
    <div class="row" v-else>
      <div class="column">
        <div class="card" id="favorites">
          <div class="card-title">
            <h2>{{ t("favorites.title") }}</h2>
          </div>

          <div class="card-content full" v-if="store.items.length > 0">
            <table>
              <tr>
                <th>{{ t("favorites.name") }}</th>
                <th>{{ t("favorites.location") }}</th>
                <th>{{ t("favorites.size") }}</th>
                <th></th>
              </tr>

              <tr v-for="item in store.items" :key="item.path">
                <td>
                  <a
                    class="link"
                    :href="hrefOf(item)"
                    @click.prevent="open(item)"
                  >
                    <i class="material-icons">{{
                      item.isDir ? "folder" : "insert_drive_file"
                    }}</i>
                    {{ item.name }}
                  </a>
                </td>
                <td>{{ locationOf(item) }}</td>
                <td>{{ item.isDir ? "" : filesize(item.size) }}</td>
                <td class="small">
                  <button
                    class="action"
                    @click="remove(item)"
                    :aria-label="t('favorites.remove')"
                    :title="t('favorites.remove')"
                  >
                    <i class="material-icons">close</i>
                  </button>
                </td>
              </tr>
            </table>
          </div>
          <h2 class="message" v-else-if="!loading">
            <i class="material-icons">star_border</i>
            <span>{{ t("favorites.nothing") }}</span>
          </h2>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { StatusError } from "@/api/utils";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { useFavoritesStore } from "@/stores/favorites";
import { filesize } from "@/utils";
import { encodePath } from "@/utils/url";
import Errors from "@/views/Errors.vue";
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

// The "Sık kullanılanlar" page (Gezgin, K87): the files and folders the user marked, in one place.

const { t } = useI18n();
const router = useRouter();
const store = useFavoritesStore();
const $showError = inject<IToastError>("$showError")!;

const loading = ref<boolean>(true);
const error = ref<StatusError | null>(null);

const routeOf = (item: IFavorite) =>
  "/files" + encodePath(item.path) + (item.isDir ? "/" : "");

const hrefOf = (item: IFavorite) => router.resolve(routeOf(item)).href;

const open = (item: IFavorite) => router.push(routeOf(item));

// Where the item lies: its folder, or "Dosyalarım" for the top.
const locationOf = (item: IFavorite) => {
  const parent = item.path.slice(0, item.path.lastIndexOf("/"));
  return parent === "" ? t("sidebar.myFiles") : parent;
};

const remove = async (item: IFavorite) => {
  try {
    await store.remove(item.path);
  } catch (e: any) {
    $showError(e);
  }
};

onMounted(async () => {
  try {
    await store.load();
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
  } finally {
    loading.value = false;
  }
});
</script>
