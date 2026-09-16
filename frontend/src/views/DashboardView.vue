<template>
  <div class="p-6">
    <h1 class="text-4xl font-bold mb-6 text-foreground">Dashboard</h1>

    <LoadingSpinner
      v-if="loading"
      size="lg"
      message="Carregando dashboard..."
      data-testid="loading-spinner"
    />

    <div
      v-if="error && !loading"
      class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4"
      role="alert"
      data-testid="error-alert"
    >
      <p class="font-bold">Erro ao carregar dashboard</p>
      <p>{{ error }}</p>
    </div>

    <template v-if="summary && !loading">
      <!-- KPIs -->
      <div
        class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-6"
        data-testid="kpi-grid"
      >
        <Card data-testid="kpi-total-servers">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground"
              >Servidores</CardTitle
            >
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">{{ summary.servers.total }}</p>
          </CardContent>
        </Card>

        <Card data-testid="kpi-ready-servers">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground"
              >Prontos</CardTitle
            >
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold text-green-600">
              {{ summary.servers.byStatus.ready }}
            </p>
          </CardContent>
        </Card>

        <Card data-testid="kpi-connection-error">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground"
              >Com erro de conexão</CardTitle
            >
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold text-red-600">
              {{ summary.servers.withConnectionError }}
            </p>
          </CardContent>
        </Card>

        <Card data-testid="kpi-backups-30d">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground"
              >Backups (30 dias)</CardTitle
            >
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">
              {{ summary.backupRunsLast30Days.total }}
            </p>
          </CardContent>
        </Card>

        <Card data-testid="kpi-success-rate">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm font-medium text-muted-foreground"
              >Taxa de sucesso</CardTitle
            >
          </CardHeader>
          <CardContent>
            <p class="text-3xl font-bold">
              {{ summary.backupRunsLast30Days.successRatePercent.toFixed(1) }}%
            </p>
          </CardContent>
        </Card>
      </div>

      <!-- Charts -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <Card data-testid="chart-status-distribution">
          <CardHeader>
            <CardTitle>Distribuição de status dos servidores</CardTitle>
          </CardHeader>
          <CardContent>
            <div class="h-64">
              <Doughnut :data="statusChartData" :options="doughnutOptions" />
            </div>
          </CardContent>
        </Card>

        <Card data-testid="chart-daily-count">
          <CardHeader>
            <CardTitle
              >Backups por dia (últimos 30 dias, por destino)</CardTitle
            >
          </CardHeader>
          <CardContent>
            <div
              v-if="!dailyCountHasData"
              class="h-64 flex items-center justify-center text-muted-foreground"
            >
              Nenhum backup nos últimos 30 dias.
            </div>
            <div v-else class="h-64">
              <Bar :data="dailyCountChartData" :options="stackedBarOptions" />
            </div>
          </CardContent>
        </Card>

        <Card data-testid="chart-daily-bytes">
          <CardHeader>
            <CardTitle
              >Dados enviados por dia (MB, últimos 30 dias, por
              destino)</CardTitle
            >
          </CardHeader>
          <CardContent>
            <div
              v-if="!dailyBytesHasData"
              class="h-64 flex items-center justify-center text-muted-foreground"
            >
              Nenhum backup nos últimos 30 dias.
            </div>
            <div v-else class="h-64">
              <Bar :data="dailyBytesChartData" :options="stackedBarOptions" />
            </div>
          </CardContent>
        </Card>

        <Card data-testid="chart-monthly-count">
          <CardHeader>
            <CardTitle
              >Backups por mês ({{ backupStats?.monthlyThisYear.year }}, por
              destino)</CardTitle
            >
          </CardHeader>
          <CardContent>
            <div
              v-if="!monthlyCountHasData"
              class="h-64 flex items-center justify-center text-muted-foreground"
            >
              Nenhum backup no ano corrente.
            </div>
            <div v-else class="h-64">
              <Bar :data="monthlyCountChartData" :options="stackedBarOptions" />
            </div>
          </CardContent>
        </Card>

        <Card data-testid="chart-monthly-bytes" class="lg:col-span-2">
          <CardHeader>
            <CardTitle
              >Dados enviados por mês (MB,
              {{ backupStats?.monthlyThisYear.year }}, por destino)</CardTitle
            >
          </CardHeader>
          <CardContent>
            <div
              v-if="!monthlyBytesHasData"
              class="h-64 flex items-center justify-center text-muted-foreground"
            >
              Nenhum backup no ano corrente.
            </div>
            <div v-else class="h-64">
              <Bar :data="monthlyBytesChartData" :options="stackedBarOptions" />
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Attention points -->
      <Card data-testid="attention-points">
        <CardHeader>
          <CardTitle>Pontos de atenção</CardTitle>
        </CardHeader>
        <CardContent class="space-y-6">
          <div
            v-if="
              summary.attentionPoints.serversAwaitingAuthorization.length ===
                0 &&
              summary.attentionPoints.serversWithConnectionError.length === 0 &&
              summary.attentionPoints.recentFailures.length === 0
            "
            class="text-muted-foreground"
            data-testid="attention-empty"
          >
            Nenhum ponto de atenção no momento.
          </div>

          <div
            v-if="
              summary.attentionPoints.serversAwaitingAuthorization.length > 0
            "
            data-testid="attention-awaiting-auth"
          >
            <h3 class="font-semibold mb-2">
              Servidores aguardando autorização SSH
            </h3>
            <ul class="space-y-1">
              <li
                v-for="s in summary.attentionPoints
                  .serversAwaitingAuthorization"
                :key="s.id"
                class="flex items-center justify-between text-sm border-b py-1"
              >
                <RouterLink
                  :to="`/servers/${s.id}/edit`"
                  class="hover:underline"
                  >{{ s.name }}</RouterLink
                >
                <Badge variant="secondary">aguardando autorização</Badge>
              </li>
            </ul>
          </div>

          <div
            v-if="summary.attentionPoints.serversWithConnectionError.length > 0"
            data-testid="attention-connection-error"
          >
            <h3 class="font-semibold mb-2">Servidores com erro de conexão</h3>
            <ul class="space-y-1">
              <li
                v-for="s in summary.attentionPoints.serversWithConnectionError"
                :key="s.id"
                class="flex items-center justify-between text-sm border-b py-1"
              >
                <RouterLink
                  :to="`/servers/${s.id}/edit`"
                  class="hover:underline"
                  >{{ s.name }}</RouterLink
                >
                <Badge variant="destructive">{{
                  s.lastTestConnectionError ?? 'falha'
                }}</Badge>
              </li>
            </ul>
          </div>

          <div
            v-if="summary.attentionPoints.recentFailures.length > 0"
            data-testid="attention-recent-failures"
          >
            <h3 class="font-semibold mb-2">Últimas falhas de backup</h3>
            <ul class="space-y-1">
              <li
                v-for="run in summary.attentionPoints.recentFailures"
                :key="run.id"
                class="flex items-center justify-between text-sm border-b py-1"
              >
                <span>{{
                  new Date(run.createdAt).toLocaleString('pt-BR')
                }}</span>
                <Badge variant="destructive">{{
                  run.errorMessage ?? 'falha'
                }}</Badge>
              </li>
            </ul>
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Chart as ChartJS, registerables } from 'chart.js'
import { Doughnut, Bar } from 'vue-chartjs'
import * as api from '@/api'
import type {
  DashboardSummaryDTO,
  DashboardBackupStatsDTO,
  DashboardDestinationSeriesDTO,
} from '@/api'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

ChartJS.register(...registerables)

const summary = ref<DashboardSummaryDTO | null>(null)
const backupStats = ref<DashboardBackupStatsDTO | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

async function fetchDashboard(): Promise<void> {
  loading.value = true
  error.value = null

  try {
    const [summaryResult, backupStatsResult] = await Promise.all([
      api.getDashboardSummary(),
      api.getDashboardBackupStats(),
    ])
    summary.value = summaryResult
    backupStats.value = backupStatsResult
  } catch (err) {
    if (import.meta.env.DEV) {
      console.error('[DEBUG] API Error:', err)
    }
    error.value = 'Erro ao carregar dashboard. Tente novamente.'
  } finally {
    loading.value = false
  }
}

const doughnutOptions = { responsive: true, maintainAspectRatio: false }
const stackedBarOptions = {
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    x: { stacked: true },
    y: { stacked: true, beginAtZero: true },
  },
}

const statusChartData = computed(() => {
  const byStatus = summary.value?.servers.byStatus
  return {
    labels: [
      'Pronto',
      'Aguardando autorização',
      'Pendente de chave',
      'Desabilitado',
    ],
    datasets: [
      {
        data: [
          byStatus?.ready ?? 0,
          byStatus?.awaitingAuthorization ?? 0,
          byStatus?.pendingKey ?? 0,
          byStatus?.disabled ?? 0,
        ],
        backgroundColor: ['#16a34a', '#eab308', '#f97316', '#6b7280'],
      },
    ],
  }
})

// Validated categorical palette (dataviz skill, 8-hue order, CVD-safe as a
// fixed sequence — see docs/Frontend.md for the validation report). Colors
// are assigned by position in destinations, never reordered by data, so a
// given destination keeps the same color across all 4 charts and across
// re-renders. "Sem destino" always gets the fixed neutral gray reused from
// the server-status doughnut's "Desabilitado" slot, not a palette color.
const DESTINATION_PALETTE = [
  '#2a78d6', // blue
  '#eb6834', // orange
  '#1baf7a', // aqua
  '#eda100', // yellow
  '#e87ba4', // magenta
  '#008300', // green
  '#4a3aa7', // violet
  '#e34948', // red
]
const NO_DESTINATION_COLOR = '#6b7280'

const destinationColors = computed<Record<string, string>>(() => {
  const dests = backupStats.value?.destinations ?? []
  const colors: Record<string, string> = {}
  let paletteIndex = 0
  for (const d of dests) {
    const key = d.storageTargetId ?? 'none'
    colors[key] =
      d.storageTargetId === null
        ? NO_DESTINATION_COLOR
        : DESTINATION_PALETTE[paletteIndex++ % DESTINATION_PALETTE.length]
  }
  return colors
})

function toStackedBarData(
  labels: string[],
  series: DashboardDestinationSeriesDTO[],
  scale = 1
) {
  const destinations = backupStats.value?.destinations ?? []
  return {
    labels,
    datasets: series.map((s) => {
      const key = s.storageTargetId ?? 'none'
      const dest = destinations.find(
        (d) => (d.storageTargetId ?? 'none') === key
      )
      return {
        label: dest?.name ?? 'Sem destino',
        data: s.data.map((v) => v / scale),
        backgroundColor: destinationColors.value[key],
      }
    }),
  }
}

const BYTES_PER_MB = 1024 * 1024

// dailyLast30Days/monthlyThisYear labels/series are always fully populated
// and zero-filled by the API (30 daily labels, 12 monthly labels, one entry
// per destination) even when there are no runs at all — so "no data" must be
// detected by checking whether every value is zero, not by an empty array.
function seriesHasData(
  series: DashboardDestinationSeriesDTO[] | undefined
): boolean {
  return (series ?? []).some((s) => s.data.some((v) => v > 0))
}

const dailyCountHasData = computed(() =>
  seriesHasData(backupStats.value?.dailyLast30Days.countSeries)
)
const dailyBytesHasData = computed(() =>
  seriesHasData(backupStats.value?.dailyLast30Days.bytesSeries)
)
const monthlyCountHasData = computed(() =>
  seriesHasData(backupStats.value?.monthlyThisYear.countSeries)
)
const monthlyBytesHasData = computed(() =>
  seriesHasData(backupStats.value?.monthlyThisYear.bytesSeries)
)

const dailyCountChartData = computed(() =>
  backupStats.value
    ? toStackedBarData(
        backupStats.value.dailyLast30Days.labels,
        backupStats.value.dailyLast30Days.countSeries
      )
    : { labels: [], datasets: [] }
)
const dailyBytesChartData = computed(() =>
  backupStats.value
    ? toStackedBarData(
        backupStats.value.dailyLast30Days.labels,
        backupStats.value.dailyLast30Days.bytesSeries,
        BYTES_PER_MB
      )
    : { labels: [], datasets: [] }
)
const monthlyCountChartData = computed(() =>
  backupStats.value
    ? toStackedBarData(
        backupStats.value.monthlyThisYear.labels,
        backupStats.value.monthlyThisYear.countSeries
      )
    : { labels: [], datasets: [] }
)
const monthlyBytesChartData = computed(() =>
  backupStats.value
    ? toStackedBarData(
        backupStats.value.monthlyThisYear.labels,
        backupStats.value.monthlyThisYear.bytesSeries,
        BYTES_PER_MB
      )
    : { labels: [], datasets: [] }
)

onMounted(() => {
  fetchDashboard()
})
</script>
