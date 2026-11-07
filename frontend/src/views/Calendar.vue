<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NButton, NButtonGroup, NEmpty, NTag, NSpin } from 'naive-ui'
import { useRouter } from 'vue-router'
import { ScheduleService } from '../lib/api'
import type { Day } from '../../bindings/github.com/aisreyu/worktools/internal/holiday/models'
import { Kind } from '../../bindings/github.com/aisreyu/worktools/internal/holiday/models'
import type { ScheduleEntry } from '../../bindings/github.com/aisreyu/worktools/internal/store/models'
import { locale, t } from '../lib/i18n'
import { lunarDayLabel } from '../lib/lunar'

const router = useRouter()

const loading = ref(false)
// special days keyed by yyyy-mm-dd
const holidays = ref<Record<string, Day>>({})
const schedules = ref<ScheduleEntry[]>([])

const now = new Date()
const year = ref(now.getFullYear())
const month = ref(now.getMonth() + 1) // 1-12
const selected = ref<Date>(new Date(now.getFullYear(), now.getMonth(), now.getDate()))

async function loadMonth() {
  loading.value = true
  try {
    const ts = Math.floor(new Date(year.value, month.value - 1, 15).getTime() / 1000)
    const days = (await ScheduleService.HolidayMonth(ts)) ?? []
    const map: Record<string, Day> = {}
    for (const d of days) {
      if (d.kind === Kind.KindNone) continue
      map[d.date.slice(0, 10)] = d
    }
    holidays.value = map
  } finally {
    loading.value = false
  }
}

async function loadSchedules() {
  schedules.value = (await ScheduleService.List()) ?? []
}

onMounted(() => {
  loadMonth()
  loadSchedules()
})
watch([year, month], loadMonth)

function prevMonth() {
  month.value--
  if (month.value < 1) {
    month.value = 12
    year.value--
  }
}

function nextMonth() {
  month.value++
  if (month.value > 12) {
    month.value = 1
    year.value++
  }
}

function goToday() {
  const d = new Date()
  year.value = d.getFullYear()
  month.value = d.getMonth() + 1
  selected.value = new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

function fmtKey(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

// schedules keyed by yyyy-mm-dd
const schedulesByDay = computed(() => {
  const map: Record<string, ScheduleEntry[]> = {}
  for (const s of schedules.value) {
    const d = new Date(s.dueAt * 1000)
    const key = fmtKey(d)
    ;(map[key] ??= []).push(s)
  }
  return map
})

interface Cell {
  date: Date
  inMonth: boolean
  holiday?: Day
  lunar: string
  schedules: ScheduleEntry[]
  isToday: boolean
}

const cells = computed<Cell[]>(() => {
  const first = new Date(year.value, month.value - 1, 1)
  // Monday-first week
  const startOffset = (first.getDay() + 6) % 7
  const start = new Date(year.value, month.value - 1, 1 - startOffset)
  const todayKey = fmtKey(new Date())
  const list: Cell[] = []
  for (let i = 0; i < 42; i++) {
    const date = new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)
    const key = fmtKey(date)
    list.push({
      date,
      inMonth: date.getMonth() === month.value - 1,
      holiday: holidays.value[key],
      lunar: lunarDayLabel(date),
      schedules: schedulesByDay.value[key] ?? [],
      isToday: key === todayKey,
    })
  }
  return list
})

const weekdayLabels = computed(() => {
  const s = t('calendar.weekdays')
  return [...s]
})

const monthLabel = computed(() => {
  const d = new Date(year.value, month.value - 1, 1)
  return d.toLocaleDateString(locale.value === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: 'long',
  })
})

const holidayCount = computed(
  () => Object.values(holidays.value).filter((d) => d.kind === Kind.KindHoliday).length,
)
const workdayCount = computed(
  () => Object.values(holidays.value).filter((d) => d.kind === Kind.KindWorkday).length,
)

const selectedKey = computed(() => fmtKey(selected.value))
const selectedHoliday = computed(() => holidays.value[selectedKey.value])
const selectedSchedules = computed(() => schedulesByDay.value[selectedKey.value] ?? [])

function dayName(d: Day): string {
  return locale.value === 'en' ? d.nameEn || d.name : d.name
}

function selectCell(c: Cell) {
  selected.value = c.date
}

function fmtTime(unix: number): string {
  return new Date(unix * 1000).toLocaleString(locale.value === 'en' ? 'en-US' : 'zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const PALETTE = ['#2080f0', '#f0a020', '#9254de', '#e88080', '#18a058', '#0fa0ce']

function eventStyle(id: number) {
  const hex = PALETTE[Math.abs(id) % PALETTE.length]
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return { color: hex, background: `rgba(${r},${g},${b},0.14)`, borderColor: hex }
}

function goSchedule() {
  router.push('/schedule')
}
</script>

<template>
  <div class="page">
    <div class="cal-header">
      <div class="cal-title">
        <h2 class="page-title">{{ t('calendar.title') }}</h2>
        <div class="legend">
          <n-tag type="error" size="small" :bordered="false"
            >{{ t('calendar.legendHoliday') }} × {{ holidayCount }}</n-tag
          >
          <n-tag type="success" size="small" :bordered="false"
            >{{ t('calendar.legendWorkday') }} × {{ workdayCount }}</n-tag
          >
        </div>
      </div>
      <div class="cal-nav">
        <n-button-group>
          <n-button size="small" @click="prevMonth">‹</n-button>
          <n-button size="small" @click="goToday">{{ t('calendar.today') }}</n-button>
          <n-button size="small" @click="nextMonth">›</n-button>
        </n-button-group>
        <span class="month-label">{{ monthLabel }}</span>
      </div>
    </div>

    <n-spin :show="loading">
      <div class="cal-body">
        <div class="cal-grid">
          <div class="weekday-row">
            <div v-for="(w, i) in weekdayLabels" :key="i" class="weekday">{{ w }}</div>
          </div>
          <div class="day-grid">
            <div
              v-for="c in cells"
              :key="c.date.getTime()"
              class="day-cell"
              :class="{
                'out-month': !c.inMonth,
                today: c.isToday,
                selected: fmtKey(c.date) === selectedKey,
                holiday: c.holiday?.kind === Kind.KindHoliday,
                workday: c.holiday?.kind === Kind.KindWorkday,
              }"
              @click="selectCell(c)"
            >
              <div class="day-top">
                <span class="day-num" :class="{ 'today-num': c.isToday }">{{ c.date.getDate() }}</span>
                <span
                  v-if="c.holiday?.kind === Kind.KindHoliday"
                  class="corner-tag holiday-tag"
                  >{{ t('calendar.holidayTag') }}</span
                >
                <span
                  v-else-if="c.holiday?.kind === Kind.KindWorkday"
                  class="corner-tag workday-tag"
                  >{{ t('calendar.workdayTag') }}</span
                >
              </div>
              <div class="day-lunar" :title="c.holiday ? dayName(c.holiday) : ''">
                {{ c.holiday ? dayName(c.holiday) : c.lunar }}
              </div>
              <div v-if="c.schedules.length" class="day-events">
                <div
                  v-for="s in c.schedules.slice(0, 2)"
                  :key="s.id"
                  class="event-pill"
                  :class="{ done: s.done }"
                  :style="eventStyle(s.id)"
                  :title="s.title"
                >
                  <span class="event-title">{{ s.title }}</span>
                </div>
                <div v-if="c.schedules.length > 2" class="event-more">
                  +{{ c.schedules.length - 2 }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="day-panel">
          <div class="panel-date">
            <div class="panel-date-num">{{ selected.getDate() }}</div>
            <div>
              <div class="panel-date-full">
                {{ selected.toLocaleDateString(locale === 'en' ? 'en-US' : 'zh-CN', { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' }) }}
              </div>
              <div class="panel-date-lunar">
                {{ lunarDayLabel(selected) }}
                <n-tag
                  v-if="selectedHoliday"
                  size="tiny"
                  :type="selectedHoliday.kind === Kind.KindHoliday ? 'error' : 'success'"
                  :bordered="false"
                  >{{ dayName(selectedHoliday) }}</n-tag
                >
              </div>
            </div>
          </div>
          <div class="panel-schedules">
            <div class="panel-sub">{{ t('calendar.daySchedules') }}</div>
            <n-empty
              v-if="!selectedSchedules.length"
              :description="t('calendar.noSchedule')"
              size="small"
              style="margin: 12px 0"
            />
            <div v-else class="panel-list">
              <div v-for="s in selectedSchedules" :key="s.id" class="panel-item">
                <span class="item-dot" :style="eventStyle(s.id)"></span>
                <span class="item-time">{{ fmtTime(s.dueAt) }}</span>
                <span class="item-title" :class="{ done: s.done }">{{ s.title }}</span>
              </div>
            </div>
            <n-button size="small" dashed block @click="goSchedule">
              {{ t('calendar.goSchedule') }}
            </n-button>
          </div>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<style scoped>
.page {
  padding: 16px;
}
.cal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
  gap: 12px;
  flex-wrap: wrap;
}
.page-title {
  margin: 0 0 8px;
  font-size: 18px;
}
.legend {
  display: flex;
  gap: 8px;
}
.cal-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}
.month-label {
  font-size: 16px;
  font-weight: 600;
  min-width: 120px;
  text-align: center;
}
.cal-body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  flex-wrap: wrap;
}
.cal-grid {
  flex: 1 1 640px;
  min-width: 560px;
}
.weekday-row {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  margin-bottom: 4px;
}
.weekday {
  text-align: center;
  font-size: 12px;
  opacity: 0.6;
  padding: 4px 0;
}
.day-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px;
}
.day-cell {
  min-height: 86px;
  border-radius: 6px;
  padding: 6px 8px;
  cursor: pointer;
  border: 1px solid rgba(128, 128, 128, 0.16);
  transition: background 0.15s;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.day-cell:hover {
  background: rgba(128, 128, 128, 0.1);
}
.day-cell.out-month {
  opacity: 0.35;
}
.day-cell.today {
  border-color: #18a058;
  box-shadow: inset 0 0 0 1px #18a058;
}
.day-cell.selected {
  background: rgba(24, 160, 88, 0.12);
}
.day-cell.holiday .day-num {
  color: #e88080;
}
.day-cell.workday .day-num {
  color: #63e2b7;
}
.day-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.day-num {
  font-size: 16px;
  font-weight: 600;
}
.day-num.today-num {
  background: #18a058;
  color: #fff;
  border-radius: 50%;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.corner-tag {
  font-size: 10px;
  border-radius: 3px;
  padding: 0 4px;
  line-height: 16px;
}
.holiday-tag {
  background: #e88080;
  color: #fff;
}
.workday-tag {
  background: #63e2b7;
  color: #fff;
}
.day-lunar {
  font-size: 11px;
  opacity: 0.65;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.day-events {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-top: 4px;
  overflow: hidden;
}
.event-pill {
  font-size: 11px;
  line-height: 18px;
  padding: 0 6px;
  border-radius: 4px;
  border-left: 3px solid;
  white-space: nowrap;
  overflow: hidden;
  cursor: pointer;
}
.event-pill .event-title {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}
.event-pill.done {
  opacity: 0.5;
}
.event-pill.done .event-title {
  text-decoration: line-through;
}
.event-more {
  font-size: 10px;
  opacity: 0.6;
  padding-left: 4px;
}
.day-panel {
  flex: 0 1 300px;
  border: 1px solid rgba(128, 128, 128, 0.16);
  border-radius: 8px;
  padding: 16px;
  min-width: 260px;
}
.panel-date {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 16px;
}
.panel-date-num {
  font-size: 40px;
  font-weight: 700;
  line-height: 1;
  color: #18a058;
}
.panel-date-full {
  font-size: 14px;
  font-weight: 600;
}
.panel-date-lunar {
  font-size: 12px;
  opacity: 0.7;
  margin-top: 2px;
  display: flex;
  gap: 6px;
  align-items: center;
}
.panel-sub {
  font-size: 13px;
  font-weight: 600;
  opacity: 0.8;
  margin-bottom: 8px;
}
.panel-list {
  max-height: 260px;
  overflow-y: auto;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.panel-item {
  display: flex;
  gap: 8px;
  font-size: 13px;
  align-items: baseline;
}
.item-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
  align-self: center;
}
.item-time {
  opacity: 0.6;
  font-size: 12px;
  white-space: nowrap;
}
.item-title.done {
  text-decoration: line-through;
  opacity: 0.5;
}
</style>
