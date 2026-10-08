<template>
  <div class="dashboard">
    <header-bar showMenu showLogo />

    <errors v-if="error" :errorCode="error.status" />
    <div class="row" v-else>
      <div class="column">
        <div class="card" id="trash">
          <div class="card-title">
            <h2>{{ t("trash.title") }}</h2>
            <span class="small" v-if="list.count > 0">
              {{ t("trash.summary", { count: list.count }) }} ·
              {{ filesize(list.size) }}
            </span>
          </div>

          <div class="card-content full" v-if="list.items.length > 0">
            <table>
              <tr>
                <th class="small">
                  <input
                    type="checkbox"
                    :checked="allSelected"
                    @change="toggleAll"
                    :aria-label="t('buttons.selectMultiple')"
                  />
                </th>
                <th>{{ t("trash.name") }}</th>
                <th>{{ t("trash.origin") }}</th>
                <th>{{ t("trash.deleted") }}</th>
                <th>{{ t("trash.size") }}</th>
              </tr>

              <tr v-for="item in list.items" :key="item.id">
                <td class="small">
                  <input type="checkbox" :value="item.id" v-model="selected" />
                </td>
                <td>
                  <i class="material-icons">{{
                    item.isDir ? "folder" : "insert_drive_file"
                  }}</i>
                  {{ item.name }}
                </td>
                <td>{{ item.origin }}</td>
                <td :title="item.deleted">{{ fromNow(item.deleted) }}</td>
                <td>{{ filesize(item.size) }}</td>
              </tr>
            </table>
          </div>
          <h2 class="message" v-else-if="!loading">
            <i class="material-icons">delete_outline</i>
            <span>{{ t("trash.nothing") }}</span>
          </h2>

          <div class="card-action" v-if="list.items.length > 0">
            <button
              v-if="canDelete && !confirmEmpty"
              class="button button--flat button--red"
              @click="confirmEmpty = true"
            >
              {{ t("trash.empty") }}
            </button>
            <button
              v-if="canDelete && confirmEmpty"
              class="button button--flat button--red"
              @click="emptyTrash"
            >
              {{ t("trash.confirmEmpty") }}
            </button>
            <button
              v-if="canDelete"
              class="button button--flat button--red"
              :disabled="selected.length === 0"
              @click="purgeSelected"
            >
              {{ t("trash.deletePermanently") }}
            </button>
            <button
              v-if="canRestore"
              class="button button--flat"
              :disabled="selected.length === 0"
              @click="restoreSelected"
            >
              {{ t("trash.restore") }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { trash as api } from "@/api";
import { StatusError } from "@/api/utils";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { useAuthStore } from "@/stores/auth";
import { filesize } from "@/utils";
import Errors from "@/views/Errors.vue";
import dayjs from "dayjs";
import { computed, inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const authStore = useAuthStore();
const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const list = ref<ITrashList>({ items: [], count: 0, size: 0 });
const selected = ref<string[]>([]);
const loading = ref<boolean>(true);
const confirmEmpty = ref<boolean>(false);
const error = ref<StatusError | null>(null);

const canRestore = computed(() => authStore.user?.perm.create === true);
const canDelete = computed(() => authStore.user?.perm.delete === true);
const allSelected = computed(
  () =>
    list.value.items.length > 0 &&
    selected.value.length === list.value.items.length
);

const fromNow = (time: string) => dayjs(time).fromNow();

const load = async () => {
  loading.value = true;
  try {
    list.value = await api.list();
    selected.value = selected.value.filter((id) =>
      list.value.items.some((item) => item.id === id)
    );
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
  } finally {
    loading.value = false;
    confirmEmpty.value = false;
  }
};

const toggleAll = () => {
  selected.value = allSelected.value
    ? []
    : list.value.items.map((item) => item.id);
};

const restoreSelected = async () => {
  try {
    const done = await api.restore(selected.value);
    $showSuccess(t("trash.restored", { count: done.length }));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

const purgeSelected = async () => {
  try {
    await api.purge(selected.value);
    $showSuccess(t("trash.purged"));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

const emptyTrash = async () => {
  try {
    await api.empty();
    $showSuccess(t("trash.emptied"));
  } catch (e: any) {
    $showError(e);
  }
  await load();
};

onMounted(load);
</script>
