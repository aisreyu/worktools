<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NColorPicker,
  NDataTable,
  NDivider,
  NGrid,
  NGi,
  NInput,
  NInputNumber,
  NModal,
  NProgress,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSlider,
  NSpace,
  NTabPane,
  NTabs,
  NTag,
  type DataTableColumns,
  useMessage,
} from 'naive-ui'
import { Clipboard, Dialogs } from '@wailsio/runtime'
import { requestSavePath, publishSaved, savedLocationSuffix } from '../lib/saveFile'
import { ToolboxService, VaultService, type CompressOptions, type CompressResult, type ConvertOptions, type ConvertResult, type WatermarkTextOptions, type PhoneResult, type IDCardResult } from '../lib/api'
import { t, errText, detailText, cronDescText, ltLabel, presetLabel } from '../lib/i18n'

const message = useMessage()

async function copy(text: string) {
  if (!text) return
  await Clipboard.SetText(text)
  message.success(t('toolbox.copied'))
}

/* ---- encoding ---- */
const encInput = ref('')
const encOutput = ref('')

async function b64Encode() {
  try {
    encOutput.value = await ToolboxService.Base64Encode(encInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function b64Decode() {
  try {
    encOutput.value = await ToolboxService.Base64Decode(encInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function hexEncode() {
  try {
    encOutput.value = await ToolboxService.HexEncode(encInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function hexDecode() {
  try {
    encOutput.value = await ToolboxService.HexDecode(encInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- hash ---- */
const hashAlgo = ref('sha256')
const hashInput = ref('')
const hashOutput = ref('')
const hashAlgos = [
  { label: 'MD5', value: 'md5' },
  { label: 'SHA1', value: 'sha1' },
  { label: 'SHA256', value: 'sha256' },
]
async function doHash() {
  try {
    hashOutput.value = await ToolboxService.Hash(hashAlgo.value, hashInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- json ---- */
const jsonInput = ref('')
const jsonOutput = ref('')
async function jsonFormat(indent: boolean) {
  try {
    jsonOutput.value = await ToolboxService.JSONFormat(jsonInput.value, indent)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- time ---- */
const tsInput = ref('')
const tsUnit = ref<'s' | 'ms'>('s')
const tsOutput = ref('')
async function tsToTime() {
  try {
    tsOutput.value = await ToolboxService.TimeFromUnix(tsInput.value, tsUnit.value === 'ms')
  } catch (e) {
    message.error(errText(e))
  }
}
const timeInput = ref('')
const timeOutput = ref('')
async function timeToTs() {
  try {
    timeOutput.value = String(await ToolboxService.UnixFromTime(timeInput.value))
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- port scan ---- */
const scanHost = ref('127.0.0.1')
const scanPorts = ref('22,80,443,3306,6379')
const scanTimeout = ref(1500)
const scanResults = ref<{ port: number; open: boolean; latency: string; error?: string }[]>([])
const scanning = ref(false)
async function doScan() {
  const ports = scanPorts.value
    .split(/[,\s]+/)
    .map((s) => parseInt(s, 10))
    .filter((n) => !Number.isNaN(n))
  if (!scanHost.value.trim() || ports.length === 0) {
    message.warning(t('toolbox.hostPortRequired'))
    return
  }
  scanning.value = true
  try {
    scanResults.value = (await ToolboxService.PortScan(
      scanHost.value.trim(),
      ports,
      scanTimeout.value,
    )) as unknown as typeof scanResults.value
  } catch (e) {
    message.error(errText(e))
  } finally {
    scanning.value = false
  }
}

/* ---- dns ---- */
const dnsName = ref('example.com')
const dnsRecord = ref('A')
const dnsResults = ref<string[]>([])
const dnsRecords = ['A', 'CNAME', 'MX', 'TXT', 'NS'].map((v) => ({ label: v, value: v }))
async function doDNS() {
  try {
    dnsResults.value = (await ToolboxService.DNSLookup(dnsName.value.trim(), dnsRecord.value)) ?? []
  } catch (e) {
    message.error(errText(e))
    dnsResults.value = []
  }
}

/* ---- subnet ---- */
const cidr = ref('192.168.1.0/24')
const subnet = ref<{ cidr: string; network: string; mask: string; broadcast: string; firstHost: string; lastHost: string; hostCount: number; prefix: number } | null>(null)
async function doSubnet() {
  try {
    subnet.value = (await ToolboxService.ParseSubnet(cidr.value.trim())) as unknown as typeof subnet.value
  } catch (e) {
    message.error(errText(e))
    subnet.value = null
  }
}

/* ---- random ---- */
const rndLen = ref(24)
const rndCharset = ref('')
const rndOutput = ref('')
async function doRandom() {
  try {
    rndOutput.value = await ToolboxService.RandomString(rndLen.value, rndCharset.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- text ops ---- */
const sepOptions = computed(() => [
  { label: t('toolbox.sepTab'), value: 'tab' },
  { label: t('toolbox.sepComma'), value: 'comma' },
  { label: t('toolbox.sepSpace'), value: 'space' },
  { label: t('toolbox.sepPipe'), value: 'pipe' },
  { label: t('toolbox.custom'), value: 'custom' },
])
const quoteOptions = computed(() => [
  { label: t('toolbox.quoteSingle') + " '", value: "'" },
  { label: t('toolbox.quoteDouble') + ' "', value: '"' },
  { label: t('toolbox.quoteBacktick'), value: '`' },
  { label: t('toolbox.quoteParen'), value: '(' },
  { label: t('toolbox.quoteBracket'), value: '[' },
  { label: t('toolbox.quoteNone'), value: '' },
])
const txtInput = ref('')
const txtOutput = ref('')
const txtSep = ref('\t')
const txtSepCustom = ref('')
const txtQuote = ref("'")
const txtSortDedupe = ref(false)
const txtNoHeader = ref(false)
function effectiveSep(): string {
  if (txtSep.value === 'custom') return txtSepCustom.value
  if (txtSep.value === 'tab') return '\t'
  if (txtSep.value === 'comma') return ','
  if (txtSep.value === 'space') return ' '
  if (txtSep.value === 'pipe') return '|'
  return '\t'
}
async function doTranspose(direction: 'r2c' | 'c2r') {
  try {
    txtOutput.value =
      direction === 'r2c'
        ? await ToolboxService.RowsToColumns(txtInput.value, effectiveSep())
        : await ToolboxService.ColumnsToRows(txtInput.value, effectiveSep())
  } catch (e) {
    message.error(errText(e))
  }
}
async function doJoin() {
  try {
    txtOutput.value = await ToolboxService.JoinLines(txtInput.value, txtSepCustom.value || ',', txtQuote.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function doQuote() {
  try {
    txtOutput.value = await ToolboxService.QuoteLines(txtInput.value, txtSepCustom.value || ',', txtQuote.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function doDedupe() {
  try {
    txtOutput.value = await ToolboxService.DedupeLines(txtInput.value, txtSortDedupe.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function doTextToJSON() {
  try {
    txtOutput.value = await ToolboxService.TextToJSON(txtInput.value, effectiveSep(), txtNoHeader.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function doJSONToText() {
  try {
    txtOutput.value = await ToolboxService.JSONToText(txtInput.value, effectiveSep())
  } catch (e) {
    message.error(errText(e))
  }
}
function useOutputAsInput() {
  txtInput.value = txtOutput.value
  txtOutput.value = ''
}

/* ---- affix join (prefix/suffix) ---- */
const affixPrefix = ref('http://')
const affixSuffix = ref('.net')
async function doAffixJoin() {
  try {
    txtOutput.value = await ToolboxService.JoinWithAffixes(
      txtInput.value,
      affixPrefix.value,
      affixSuffix.value,
      '\n',
    )
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- password generator ---- */
const pwLevel = ref<'low' | 'medium' | 'high'>('high')
const pwLength = ref(16)
const pwUseLower = ref<boolean | null>(null)
const pwUseUpper = ref<boolean | null>(null)
const pwUseDigits = ref<boolean | null>(null)
const pwUseSymbols = ref<boolean | null>(null)
const pwCustomSymbols = ref('')
const pwNoAmbiguous = ref(false)
const pwOutput = ref('')
const pwLevelOptions = computed(() => [
  { label: t('toolbox.pwLevelLow'), value: 'low' },
  { label: t('toolbox.pwLevelMedium'), value: 'medium' },
  { label: t('toolbox.pwLevelHigh'), value: 'high' },
])
function resetPwOverrides() {
  pwUseLower.value = null
  pwUseUpper.value = null
  pwUseDigits.value = null
  pwUseSymbols.value = null
}

async function doGeneratePassword() {
  try {
    pwOutput.value = await ToolboxService.GeneratePassword({
      level: pwLevel.value,
      length: pwLength.value,
      useLower: pwUseLower.value,
      useUpper: pwUseUpper.value,
      useDigits: pwUseDigits.value,
      useSymbols: pwUseSymbols.value,
      customSymbols: pwCustomSymbols.value,
      noAmbiguous: pwNoAmbiguous.value,
    })
  } catch (e) {
    message.error(errText(e))
  }
}
const pwSaveModal = ref(false)
const pwSaveTitle = ref('')
const pwSaveNote = ref('')
const pwSaving = ref(false)
function openPwSave() {
  if (!pwOutput.value) {
    message.warning(t('toolbox.pwdGenerateFirst'))
    return
  }
  pwSaveTitle.value = ''
  pwSaveNote.value = ''
  pwSaveModal.value = true
}
async function doSavePassword() {
  pwSaving.value = true
  try {
    await ToolboxService.SavePasswordToVault(pwOutput.value, pwSaveTitle.value, pwSaveNote.value)
    message.success(t('toolbox.pwdSavedVault'))
    pwSaveModal.value = false
  } catch (e) {
    message.error(errText(e))
  } finally {
    pwSaving.value = false
  }
}

/* ---- random identity data ---- */
const idCount = ref(5)
const idEmails = ref<string[]>([])
const idPhones = ref<string[]>([])
const idCards = ref<string[]>([])
async function genEmails() {
  try {
    idEmails.value = (await ToolboxService.RandomEmails(idCount.value)) ?? []
  } catch (e) {
    message.error(errText(e))
  }
}
async function genPhones() {
  try {
    idPhones.value = (await ToolboxService.RandomPhones(idCount.value)) ?? []
  } catch (e) {
    message.error(errText(e))
  }
}
async function genIDCards() {
  try {
    idCards.value = (await ToolboxService.RandomIDCards(idCount.value)) ?? []
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- phone / id-card lookup (offline) ---- */
const phInput = ref('')
const phResult = ref<PhoneResult | null>(null)
const phLoading = ref(false)
async function lookupPhone() {
  if (!phInput.value.trim()) return
  phLoading.value = true
  try {
    phResult.value = await ToolboxService.LookupPhone(phInput.value.trim())
  } catch (e) {
    message.error(errText(e))
  } finally {
    phLoading.value = false
  }
}

const idInput = ref('')
const idResult = ref<IDCardResult | null>(null)
const idLoading = ref(false)
async function lookupIDCard() {
  if (!idInput.value.trim()) return
  idLoading.value = true
  try {
    idResult.value = await ToolboxService.LookupIDCard(idInput.value.trim())
  } catch (e) {
    message.error(errText(e))
  } finally {
    idLoading.value = false
  }
}

/* ---- ssl check ---- */
type CertInfo = {
  subject: string
  issuer: string
  serialNumber: string
  notBefore: string
  notAfter: string
  dnsNames: string[]
  signatureAlgorithm: string
  publicKeyAlgorithm: string
  publicKeyBits: number
  isCA: boolean
  selfSigned: boolean
}
type ProtocolInfo = { version: string; supported: boolean; cipherName?: string; error?: string }
type SSLResult = {
  host: string
  port: number
  ip: string
  tlsVersion: string
  cipherSuite: string
  latency: string
  chain: CertInfo[]
  leaf: CertInfo
  daysRemaining: number
  expired: boolean
  expiringSoon: boolean
  protocols: ProtocolInfo[]
  issues: string[]
  score: number
  grade: string
}
const sslHost = ref('www.baidu.com')
const sslPort = ref(443)
const sslTimeout = ref(8000)
const sslResult = ref<SSLResult | null>(null)
const sslChecking = ref(false)
async function doSSLCheck() {
  if (!sslHost.value.trim()) {
    message.warning(t('toolbox.sslDomainRequired'))
    return
  }
  sslChecking.value = true
  sslResult.value = null
  try {
    sslResult.value = (await ToolboxService.SSLCheck(
      sslHost.value.trim(),
      sslPort.value,
      sslTimeout.value,
    )) as unknown as SSLResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    sslChecking.value = false
  }
}
function gradeColor(grade: string): string {
  if (grade.startsWith('A')) return '#18a058'
  if (grade.startsWith('B')) return '#2080f0'
  if (grade.startsWith('C')) return '#f0a020'
  return '#d03050'
}
function gradeType(grade: string): 'success' | 'info' | 'warning' | 'error' {
  if (grade.startsWith('A')) return 'success'
  if (grade.startsWith('B')) return 'info'
  if (grade.startsWith('C')) return 'warning'
  return 'error'
}
function sslIssueText(issue: string): string {
  const idx = issue.indexOf(':')
  const code = idx > 0 ? issue.slice(0, idx) : issue
  const arg = idx > 0 ? issue.slice(idx + 1) : ''
  const n = parseInt(arg, 10)
  if (code === 'hostname_mismatch') return t('sslIssue.hostname_mismatch') + (arg ? `: ${arg}` : '')
  if (code === 'expired' || code === 'expiring') return t(`sslIssue.${code}`, { n: Number.isFinite(n) ? n : 0 })
  const key = `sslIssue.${code}`
  return t(key) !== key ? t(key) : issue
}

function k8sIssueText(issue: string): string {
  // Codes: k8s_line_error:LINE:scope.key:val:err | k8s_limit_below_request:res:lim:req
  const parts = issue.split(':')
  const code = parts[0]
  if (code === 'k8s_line_error') {
    const line = parts[1] ?? ''
    const scope = parts[2] ?? ''
    const key = parts[3] ?? ''
    const val = parts[4] ?? ''
    const err = parts.slice(5).join(':')
    return t('err.k8s_line_error', { n: line, v: `${scope}.${key} = "${val}": ${err}` })
  }
  if (code === 'k8s_limit_below_request') {
    const res = parts[1] ?? ''
    const lim = parts[2] ?? ''
    const req = parts[3] ?? ''
    return t('err.k8s_limit_below_request', { v: `${res} (${lim} < ${req})` })
  }
  return errText(issue)
}

function certName(c: CertInfo): string {
  // "CN=baidu.com,O=..." → "baidu.com"
  const m = c.subject.match(/CN=([^,]+)/)
  return m ? m[1] : c.subject
}
function issuerName(c: CertInfo): string {
  const m = c.issuer.match(/CN=([^,]+)/)
  return m ? m[1] : c.issuer
}

/* ---- http probe (enhanced / lightweight curl) ---- */
type HTTPTimings = { dnsMs: number; connMs: number; tlsMs: number; ttfbMs: number; totalMs: number }
type HTTPProbeResult = {
  status: number
  statusText: string
  latency: string
  latencyMs: number
  proto: string
  headers: Record<string, string>
  bodySnippet: string
  bodySize: number
  attempts: number
  finalUrl: string
  redirects: string[]
  timings: HTTPTimings
}
const hreqMethod = ref('GET')
const hreqURL = ref('https://www.baidu.com')
const hreqHeaders = ref('')
const hreqBody = ref('')
const hreqTimeout = ref(10000)
const hreqRetries = ref(0)
const hreqRetryDelay = ref(300)
const hreqFollow = ref(true)
const hreqMaxBody = ref(4096)
const hreqResult = ref<HTTPProbeResult | null>(null)
const hreqRunning = ref(false)
const hreqMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].map((v) => ({ label: v, value: v }))

function parseHeaderLines(text: string) {
  const out: { key: string; value: string }[] = []
  for (const ln of text.split('\n')) {
    const t = ln.trim()
    if (!t) continue
    const idx = t.indexOf(':')
    if (idx <= 0) continue
    out.push({ key: t.slice(0, idx).trim(), value: t.slice(idx + 1).trim() })
  }
  return out
}
async function doHTTPRequest() {
  if (!hreqURL.value.trim()) {
    message.warning(t('toolbox.urlRequired'))
    return
  }
  hreqRunning.value = true
  hreqResult.value = null
  try {
    hreqResult.value = (await ToolboxService.HTTPRequest({
      url: hreqURL.value.trim(),
      method: hreqMethod.value,
      headers: parseHeaderLines(hreqHeaders.value),
      body: hreqBody.value,
      timeoutMs: hreqTimeout.value,
      retries: hreqRetries.value,
      retryDelayMs: hreqRetryDelay.value,
      followRedirects: hreqFollow.value,
      maxBodyBytes: hreqMaxBody.value,
    })) as unknown as HTTPProbeResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    hreqRunning.value = false
  }
}

/* ---- port presets ---- */
type PortPreset = { name: string; label: string; spec: string; count: number }
const portPresets = ref<PortPreset[]>([])
onMounted(async () => {
  try {
    portPresets.value = ((await ToolboxService.PortPresets()) ?? []) as unknown as PortPreset[]
  } catch {
    portPresets.value = []
  }
})
async function applyPreset(spec: string) {
  scanPorts.value = spec
  try {
    const ports = await ToolboxService.ExpandPortSpec(spec)
    scanPorts.value = (ports ?? []).join(',')
  } catch (e) {
    message.error(errText(e))
  }
}
async function expandPortSpecText() {
  try {
    const ports = await ToolboxService.ExpandPortSpec(scanPorts.value)
    scanPorts.value = (ports ?? []).join(',')
    message.success(t('toolbox.expandedN', { n: (ports ?? []).length }))
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- tcping ---- */
type TCPingSample = { seq: number; open: boolean; latencyMs: number; latency: string; error?: string }
type TCPingResult = {
  host: string
  port: number
  sent: number
  received: number
  lossPercent: number
  min: string
  avg: string
  max: string
  samples: TCPingSample[]
}
const tcHost = ref('127.0.0.1')
const tcPort = ref(443)
const tcCount = ref(4)
const tcTimeout = ref(2000)
const tcInterval = ref(200)
const tcResult = ref<TCPingResult | null>(null)
const tcRunning = ref(false)
async function doTCPing() {
  if (!tcHost.value.trim()) {
    message.warning(t('toolbox.hostRequired'))
    return
  }
  tcRunning.value = true
  tcResult.value = null
  try {
    tcResult.value = (await ToolboxService.TCPing(
      tcHost.value.trim(),
      tcPort.value,
      tcCount.value,
      tcTimeout.value,
      tcInterval.value,
    )) as unknown as TCPingResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    tcRunning.value = false
  }
}

/* ---- whois ---- */
type WhoisResult = {
  query: string
  server: string
  raw: string
  fields: Record<string, string>
  registrar: string
  created: string
  updated: string
  expires: string
  nameServers: string[]
  status: string[]
  registrant: string
  daysToExpiry: number
  expiryKnown: boolean
}
const whoisDomain = ref('example.com')
const whoisTimeout = ref(10000)
const whoisResult = ref<WhoisResult | null>(null)
const whoisRunning = ref(false)
const whoisShowRaw = ref(false)
async function doWhois() {
  if (!whoisDomain.value.trim()) {
    message.warning(t('toolbox.domainRequired'))
    return
  }
  whoisRunning.value = true
  whoisResult.value = null
  try {
    whoisResult.value = (await ToolboxService.Whois(
      whoisDomain.value.trim(),
      whoisTimeout.value,
    )) as unknown as WhoisResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    whoisRunning.value = false
  }
}

/* ---- ip lookup ---- */
type IPLookupResult = {
  input: string
  ok: boolean
  message: string
  query: string
  country: string
  countryCode: string
  region: string
  city: string
  zip: string
  lat: number
  lon: number
  timezone: string
  isp: string
  org: string
  as: string
}
const ipQuery = ref('')
const ipBatchText = ref('')
const ipBatchMode = ref(false)
const ipTimeout = ref(8)
const ipTimeoutMS = computed(() => Math.round(ipTimeout.value * 1000))
const ipResult = ref<IPLookupResult | null>(null)
const ipBatchResults = ref<IPLookupResult[]>([])
const ipRunning = ref(false)
async function doIPLookup() {
  if (!ipQuery.value.trim()) {
    message.warning(t('toolbox.ipRequired'))
    return
  }
  ipRunning.value = true
  ipResult.value = null
  try {
    ipResult.value = (await ToolboxService.IPLookup(
      ipQuery.value.trim(),
      ipTimeoutMS.value,
    )) as unknown as IPLookupResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    ipRunning.value = false
  }
}
async function doIPLookupBatch() {
  const list = ipBatchText.value
    .split(/[\n,;]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  if (!list.length) {
    message.warning(t('toolbox.ipBatchRequired'))
    return
  }
  ipRunning.value = true
  ipBatchResults.value = []
  try {
    ipBatchResults.value = (await ToolboxService.IPLookupBatch(
      list,
      ipTimeoutMS.value,
    )) as unknown as IPLookupResult[]
  } catch (e) {
    message.error(errText(e))
  } finally {
    ipRunning.value = false
  }
}
function ipLocation(r: IPLookupResult): string {
  return [r.country, r.region, r.city].filter(Boolean).join(' / ')
}
type IPBatchRow = {
  input: string
  location: string
  isp: string
  as: string
  status: string
  _ok: boolean
}
const ipBatchColumns = computed<DataTableColumns<IPBatchRow>>(() => [
  { title: t('toolbox.ipColInput'), key: 'input', width: 160 },
  { title: t('toolbox.ipColLocation'), key: 'location', minWidth: 180 },
  { title: t('toolbox.ipColIsp'), key: 'isp', minWidth: 160 },
  { title: t('toolbox.ipColAs'), key: 'as', minWidth: 150 },
  {
    title: t('toolbox.ipColStatus'),
    key: 'status',
    width: 130,
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: row._ok ? 'success' : 'warning', bordered: false },
        { default: () => row.status },
      ),
  },
])
const ipBatchRows = computed<IPBatchRow[]>(() =>
  ipBatchResults.value.map((r) => ({
    input: r.query || r.input,
    location: r.ok ? ipLocation(r) : '-',
    isp: r.isp || '-',
    as: r.as || r.org || '-',
    status: r.ok ? t('toolbox.ipOk') : r.message || t('toolbox.ipFail'),
    _ok: r.ok,
  })),
)

/* ---- certgen ---- */
type CertGenResult = {
  certPem: string
  keyPem: string
  subject: string
  notBefore: string
  notAfter: string
  keyType: string
  sans: string[]
  serial: string
}
const cgCN = ref('example.local')
const cgOrg = ref('WorkTools')
const cgOU = ref('Dev')
const cgCountry = ref('CN')
const cgDNS = ref('example.local, www.example.local')
const cgIPs = ref('127.0.0.1')
const cgDays = ref(365)
const cgKeyType = ref('rsa2048')
const cgIsCA = ref(false)
const cgResult = ref<CertGenResult | null>(null)
const cgRunning = ref(false)
const cgKeyTypes = [
  { label: 'RSA 2048', value: 'rsa2048' },
  { label: 'RSA 4096', value: 'rsa4096' },
  { label: 'ECDSA P-256', value: 'ecdsa256' },
  { label: 'ECDSA P-384', value: 'ecdsa384' },
]
function splitList(s: string) {
  return s.split(/[,\s]+/).map((x) => x.trim()).filter(Boolean)
}
function certGenOpts() {
  return {
    commonName: cgCN.value.trim(),
    organization: cgOrg.value.trim(),
    orgUnit: cgOU.value.trim(),
    country: cgCountry.value.trim(),
    dnsNames: splitList(cgDNS.value),
    ips: splitList(cgIPs.value),
    days: cgDays.value,
    keyType: cgKeyType.value,
    isCa: cgIsCA.value,
  }
}
async function doGenerateCert() {
  cgRunning.value = true
  cgResult.value = null
  try {
    cgResult.value = (await ToolboxService.GenerateSelfSignedCert(certGenOpts())) as unknown as CertGenResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    cgRunning.value = false
  }
}
async function doGenerateCSR() {
  cgRunning.value = true
  cgResult.value = null
  try {
    cgResult.value = (await ToolboxService.GenerateCSR(certGenOpts())) as unknown as CertGenResult
  } catch (e) {
    message.error(errText(e))
  } finally {
    cgRunning.value = false
  }
}
function downloadText(name: string, text: string) {
  const blob = new Blob([text], { type: 'text/plain' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = name
  a.click()
  URL.revokeObjectURL(a.href)
}

/* ---- connection string ---- */
type ConnString = {
  kind: string
  scheme: string
  host: string
  port: number
  database: string
  username: string
  password: string
  params: Record<string, string>
  raw: string
}
const csRaw = ref('mysql://root:secret@127.0.0.1:3306/app?charset=utf8mb4')
const csParsed = ref<ConnString | null>(null)
const csKind = ref('mysql')
const csHost = ref('127.0.0.1')
const csPort = ref(3306)
const csDatabase = ref('app')
const csUser = ref('root')
const csPassword = ref('')
const csParams = ref('')
const csBuilt = ref('')
const csKinds = ['mysql', 'postgres', 'redis', 'kafka', 'mongodb', 'amqp'].map((v) => ({ label: v, value: v }))
async function doParseConn() {
  try {
    csParsed.value = (await ToolboxService.ParseConnString(csRaw.value.trim())) as unknown as ConnString
  } catch (e) {
    message.error(errText(e))
    csParsed.value = null
  }
}
async function doBuildConn() {
  const params: Record<string, string> = {}
  for (const pair of csParams.value.split(/[&\n]+/)) {
    const t = pair.trim()
    if (!t) continue
    const idx = t.indexOf('=')
    if (idx <= 0) continue
    params[t.slice(0, idx).trim()] = t.slice(idx + 1).trim()
  }
  try {
    csBuilt.value = await ToolboxService.BuildConnString({
      kind: csKind.value,
      scheme: '',
      host: csHost.value.trim(),
      port: csPort.value,
      database: csDatabase.value.trim(),
      username: csUser.value.trim(),
      password: csPassword.value,
      params,
      raw: '',
    })
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- log time ---- */
type LogTimeFormat = { id: string; label: string; layout: string; regex: string; hasTz: boolean }
type LogTimeMatch = { start: number; end: number; text: string; unix: number; unixMs: number; rfc3339: string; formatId: string }
type LogTimeResult = { formatId: string; format: string; matches: LogTimeMatch[]; converted: string; count: number }
const ltInput = ref('2024-01-01 12:00:00 INFO start\n2024-01-01 12:00:05 WARN slow')
const ltOutput = ref('2024-01-01 15:04:05')
const ltDetected = ref('')
const ltResult = ref<LogTimeResult | null>(null)
const ltFormatOverride = ref('')
const ltFormatOptions = computed(() => [
  { label: t('toolbox.ltAuto'), value: '' },
  { label: '2024-01-01 15:04:05', value: 'datetime' },
  { label: 'RFC3339', value: 'rfc3339' },
  { label: 'Apache', value: 'apache' },
  { label: t('toolbox.ltUnixS'), value: 'epoch_s' },
  { label: t('toolbox.ltUnixMs'), value: 'epoch_ms' },
])
const ltOutputOptions = computed(() => [
  { label: t('toolbox.ltOutLocal'), value: 'local' },
  { label: 'RFC3339', value: 'rfc3339' },
  { label: t('toolbox.ltOutEpochS'), value: 'epoch' },
  { label: t('toolbox.ltOutEpochMs'), value: 'epoch_ms' },
])
async function doDetectLogTime() {
  try {
    const f = (await ToolboxService.DetectLogTime(ltInput.value)) as unknown as LogTimeFormat
    ltDetected.value = f.label
    if (f.id) ltFormatOverride.value = f.id
  } catch (e) {
    message.error(errText(e))
  }
}
async function doConvertLogTime() {
  try {
    ltResult.value = (await ToolboxService.ConvertLogTime(
      ltInput.value,
      ltFormatOverride.value,
      ltOutput.value,
    )) as unknown as LogTimeResult
  } catch (e) {
    message.error(errText(e))
    ltResult.value = null
  }
}

/* ---- k8s calculator ---- */
type K8sResourceEntry = {
  scope: string
  resource: string
  raw: string
  cpu?: string
  cpuPlain?: string
  memory?: string
  memoryBytes?: number
  valid: boolean
  note?: string
}
type K8sValidationResult = { entries: K8sResourceEntry[]; issues: string[]; ok: boolean }
const k8sInput = ref('500m')
const k8sToUnit = ref('cpu')
const k8sConverted = ref('')
const k8sConvertUnitOptions = computed(() => {
  if (k8sInput.value.trim().toLowerCase().endsWith('m') || /^\d+(\.\d+)?$/.test(k8sInput.value.trim())) {
    return [
      { label: t('toolbox.k8sUnitCpu'), value: 'cpu' },
      { label: t('toolbox.k8sUnitM'), value: 'm' },
      { label: 'Ki', value: 'Ki' },
      { label: 'Mi', value: 'Mi' },
      { label: 'Gi', value: 'Gi' },
    ]
  }
  return [
    { label: 'Ki', value: 'Ki' },
    { label: 'Mi', value: 'Mi' },
    { label: 'Gi', value: 'Gi' },
    { label: 'bytes', value: 'bytes' },
  ]
})
async function doConvertK8s() {
  try {
    k8sConverted.value = await ToolboxService.ConvertK8sResource(k8sInput.value.trim(), k8sToUnit.value)
  } catch (e) {
    message.error(errText(e))
    k8sConverted.value = ''
  }
}
const k8sYaml = ref('resources:\n  requests:\n    cpu: 250m\n    memory: 512Mi\n  limits:\n    cpu: 500m\n    memory: 1Gi')
const k8sValidate = ref<K8sValidationResult | null>(null)
async function doValidateK8s() {
  try {
    k8sValidate.value = (await ToolboxService.ValidateResourcesYAML(k8sYaml.value)) as unknown as K8sValidationResult
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- ops tools: capacity & bandwidth calculators ---- */
type UnitConversion = { value: number; unit: string; text: string; base: number; decimal: string; binary: string }
type TransferEstimate = { bytes: number; payload: string; bitsPerSec: number; linkSpeed: string; seconds: number; duration: string; sustainedBy: string }
type RAIDPlan = { level: string; disks: number; diskSize: number; rawTB: number; usableTB: number; lostTB: number; efficiency: number; tolerance: number }
type TrafficEstimate = { bandwidthMbps: number; utilization: number; effectiveMbps: number; days: number; bytesPerSec: number; perDayBytes: number; totalBytes: number; perDayText: string; totalText: string; monthlyBytes: number; monthlyText: string }

const sizeUnitOptions = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'].map((u) => ({ label: u, value: u }))
const rateUnitOptions = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps', 'B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'].map((u) => ({ label: u, value: u }))
const raidLevelOptions = ['raid0', 'raid1', 'raid5', 'raid6', 'raid10'].map((u) => ({ label: u.toUpperCase(), value: u }))

const capsSizeValue = ref(1)
const capsSizeFrom = ref('GiB')
const capsSizeTo = ref('MB')
const capsSizeResult = ref<UnitConversion | null>(null)
async function doConvertSize() {
  try {
    capsSizeResult.value = (await ToolboxService.ConvertDataSize(capsSizeValue.value, capsSizeFrom.value, capsSizeTo.value)) as unknown as UnitConversion
  } catch (e) {
    message.error(errText(e))
    capsSizeResult.value = null
  }
}

const capsRateValue = ref(100)
const capsRateFrom = ref('Mbps')
const capsRateTo = ref('MB/s')
const capsRateResult = ref<UnitConversion | null>(null)
async function doConvertRate() {
  try {
    capsRateResult.value = (await ToolboxService.ConvertDataRate(capsRateValue.value, capsRateFrom.value, capsRateTo.value)) as unknown as UnitConversion
  } catch (e) {
    message.error(errText(e))
    capsRateResult.value = null
  }
}

const capsTransferSize = ref(10)
const capsTransferSizeUnit = ref('GB')
const capsTransferSpeed = ref(100)
const capsTransferSpeedUnit = ref('Mbps')
const capsTransferEff = ref(80)
const capsTransferResult = ref<TransferEstimate | null>(null)
async function doEstimateTransfer() {
  try {
    capsTransferResult.value = (await ToolboxService.EstimateTransferTime(
      capsTransferSize.value, capsTransferSizeUnit.value, capsTransferSpeed.value, capsTransferSpeedUnit.value, capsTransferEff.value,
    )) as unknown as TransferEstimate
  } catch (e) {
    message.error(errText(e))
    capsTransferResult.value = null
  }
}

const capsRaidDisks = ref(4)
const capsRaidDiskSize = ref(4)
const capsRaidLevel = ref('raid5')
const capsRaidResult = ref<RAIDPlan | null>(null)
async function doCalcRaid() {
  try {
    capsRaidResult.value = (await ToolboxService.CalcRAIDCapacity(capsRaidDisks.value, capsRaidDiskSize.value, capsRaidLevel.value)) as unknown as RAIDPlan
  } catch (e) {
    message.error(errText(e))
    capsRaidResult.value = null
  }
}

const capsTrafficBw = ref(100)
const capsTrafficUtil = ref(50)
const capsTrafficDays = ref(30)
const capsTrafficResult = ref<TrafficEstimate | null>(null)
async function doEstimateTraffic() {
  try {
    capsTrafficResult.value = (await ToolboxService.EstimateTraffic(capsTrafficBw.value, capsTrafficUtil.value, capsTrafficDays.value)) as unknown as TrafficEstimate
  } catch (e) {
    message.error(errText(e))
    capsTrafficResult.value = null
  }
}

/* ---- daily tools: URL encode/decode ---- */
const urlInput = ref('a b&c=d')
const urlMode = ref('query')
const urlOutput = ref('')
async function urlEncode() {
  try {
    urlOutput.value = await ToolboxService.URLEncodeComponent(urlInput.value, urlMode.value)
  } catch (e) {
    message.error(errText(e))
  }
}async function urlDecode() {
  try {
    urlOutput.value = await ToolboxService.URLDecodeComponent(urlInput.value, urlMode.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- daily tools: JWT ---- */
type JWTResult = {
  valid: boolean
  header: Record<string, unknown>
  payload: Record<string, unknown>
  headerJson: string
  payloadJson: string
  signature: string
  alg: string
  issuedAt?: number
  notBefore?: number
  expiresAt?: number
  expired: boolean
  expiresIn?: string
}
const jwtInput = ref('')
const jwtResult = ref<JWTResult | null>(null)
async function decodeJWT() {
  try {
    jwtResult.value = (await ToolboxService.DecodeJWT(jwtInput.value)) as unknown as JWTResult
  } catch (e) {
    jwtResult.value = null
    message.error(errText(e))
  }
}

/* ---- daily tools: cron ---- */
type CronSchedule = {
  expression: string
  minute: string
  hour: string
  dayOfMonth: string
  month: string
  dayOfWeek: string
  description: string
  nextRuns: string[]
}
const cronExpr = ref('0 9 * * mon-fri')
const cronResult = ref<CronSchedule | null>(null)
const cronPresets = computed(() => [
  { label: t('cronDesc.every_minute'), value: '* * * * *' },
  { label: t('cronDesc.hourly', { m: '00' }), value: '0 * * * *' },
  { label: t('cronDesc.every_day', { h: '09', m: '00' }), value: '0 9 * * *' },
  { label: t('cronDesc.every_weekday', { h: '09', m: '00' }), value: '0 9 * * mon-fri' },
  { label: t('cronDesc.weekly', { d: t('toolbox.dow1'), h: '09', m: '00' }), value: '0 9 * * 1' },
  { label: t('cronDesc.monthly', { dom: '1', h: '09', m: '00' }), value: '0 9 1 * *' },
  { label: t('cronDesc.yearly', { mon: '1', dom: '1', h: '00', m: '00' }), value: '0 0 1 1 *' },
])
async function parseCron() {
  try {
    cronResult.value = (await ToolboxService.ParseCron(cronExpr.value, 8)) as unknown as CronSchedule
  } catch (e) {
    cronResult.value = null
    message.error(errText(e))
  }
}

/* ---- daily tools: YAML ---- */
const yamlInput = ref('a: 1\nb:\n  - x\n  - y')
const yamlOutput = ref('')
async function yamlFormat() {
  try {
    yamlOutput.value = await ToolboxService.FormatYAML(yamlInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function yamlValidate() {
  try {
    await ToolboxService.ValidateYAML(yamlInput.value)
    message.success(t('toolbox.yamlValid'))
  } catch (e) {
    message.error(errText(e))
  }
}
async function yamlToJSON() {
  try {
    yamlOutput.value = await ToolboxService.YAMLToJSON(yamlInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}
async function jsonToYAML() {
  try {
    yamlOutput.value = await ToolboxService.JSONToYAML(yamlInput.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- daily tools: time diff ---- */
type TimeDiffResult = {
  from: string
  to: string
  seconds: number
  humanized: string
  absHumanized: string
}
const tdFrom = ref('2024-01-01 00:00:00')
const tdTo = ref('2024-01-02 12:00:00')
const tdResult = ref<TimeDiffResult | null>(null)
async function calcTimeDiff() {
  try {
    tdResult.value = (await ToolboxService.TimeDiff(tdFrom.value, tdTo.value)) as unknown as TimeDiffResult
  } catch (e) {
    tdResult.value = null
    message.error(errText(e))
  }
}
async function tdFillNow() {
  tdTo.value = await ToolboxService.Now()
}

/* ---- daily tools: identifiers ---- */
const idKind = ref('uuid')
const newIdCount = ref(5)
const idOutput = ref('')
async function genIDs() {
  try {
    const list = idKind.value === 'ulid' ? await ToolboxService.ULIDs(newIdCount.value) : await ToolboxService.UUIDs(newIdCount.value)
    idOutput.value = (list as string[]).join('\n')
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- daily tools: regex ---- */
type RegexMatch = { index: number; text: string; groups: string[]; named?: Record<string, string> }
type RegexResult = { matches: RegexMatch[]; count: number; error?: string }
const rePattern = ref('(\\d+)-(\\d+)')
const reInput = ref('a 12-34 b 5-6')
const reIgnoreCase = ref(false)
const reResult = ref<RegexResult | null>(null)
async function runRegex() {
  reResult.value = (await ToolboxService.RegexTest(rePattern.value, reInput.value, reIgnoreCase.value)) as unknown as RegexResult
  if (reResult.value.error) message.error(reResult.value.error)
}

/* ---- daily tools: text diff ---- */
type DiffLine = { type: string; a: number; b: number; text: string }
type DiffResult = { lines: DiffLine[]; added: number; deleted: number; same: number }
const diffA = ref('a\nb\nc')
const diffB = ref('a\nx\nc')
const diffResult = ref<DiffResult | null>(null)
async function runDiff() {
  diffResult.value = (await ToolboxService.TextDiff(diffA.value, diffB.value)) as unknown as DiffResult
}

/* ---- daily tools: csv to sql ---- */
const csvInput = ref('id,name\n1,alice\n2,bob')
const csvTable = ref('my_table')
const csvDialect = ref('postgres')
const csvCreate = ref(true)
const csvOutput = ref('')
async function csvToSQL() {
  try {
    csvOutput.value = await ToolboxService.CSVToSQL(csvInput.value, csvTable.value, csvDialect.value, csvCreate.value)
  } catch (e) {
    message.error(errText(e))
  }
}

/* ---- daily tools: qr ---- */
const qrText = ref('https://example.com')
const qrLevel = ref('M')
const qrScale = ref(6)
const qrImage = ref('')
function downloadQR() {
  if (!qrImage.value) return
  const a = document.createElement('a')
  a.href = qrImage.value
  a.download = 'qrcode.png'
  a.click()
}

async function genQR() {
  try {
    const b64 = await ToolboxService.QRGenerate(qrText.value, qrLevel.value, qrScale.value)
    qrImage.value = 'data:image/png;base64,' + b64
  } catch (e) {
    qrImage.value = ''
    message.error(errText(e))
  }
}

/* ---- image compression ---- */
const imgPickPaths = ref<string[]>([])
const imgTargetKB = ref<number | null>(null)
const imgMaxDim = ref<number | null>(null)
const imgQuality = ref<number | null>(null)
const imgFormat = ref('auto')
const imgRunning = ref(false)
const imgResults = ref<CompressResult[]>([])

/* ---- folder compression: one folder -> one ZIP archive ---- */
const folderZipPaths = ref<string[]>([])
const folderZipRunning = ref(false)
const folderZipResult = ref('')

const imgFormatOptions = [
  { label: t('toolbox.imgFormatAuto'), value: 'auto' },
  { label: t('toolbox.imgFormatJpeg'), value: 'jpeg' },
  { label: t('toolbox.imgFormatPng'), value: 'png' },
]

function imgBuildOpts(): CompressOptions {
  return {
    TargetKB: imgTargetKB.value ?? 0,
    MaxDim: imgMaxDim.value ?? 0,
    Quality: imgQuality.value ?? 0,
    Format: imgFormat.value,
  }
}

async function imgPick() {
  const picked = await Dialogs.OpenFile({
    Title: t('toolbox.imgPick'),
    AllowsMultipleSelection: true,
    Filters: [{ DisplayName: t('toolbox.imgTab'), Pattern: '*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.webp;*.tif;*.tiff' }],
  })
  const paths = Array.isArray(picked) ? picked : picked ? [picked] : []
  if (paths.length === 0) return
  imgPickPaths.value = paths
  imgResults.value = []
}

async function imgRun() {
  if (imgPickPaths.value.length === 0) {
    message.warning(t('toolbox.imgEmpty'))
    return
  }
  imgRunning.value = true
  try {
    // Preview run: only metadata, nothing written to disk.
    imgResults.value = ((await ToolboxService.CompressBatch(imgPickPaths.value, '', imgBuildOpts())) ?? []) as CompressResult[]
    message.success(t('toolbox.imgDone'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    imgRunning.value = false
  }
}

// imgSaveOne stores a single result next to its source (no folder needed).
async function imgSaveOne(i: number) {
  const src = imgPickPaths.value[i]
  const res = imgResults.value[i]
  if (!src || !res) return
  const ext = res.mime === 'image/png' ? '.png' : res.mime === 'image/webp' ? '.webp' : '.jpg'
  const base = src.replace(/\\/g, '/').split('/').pop() ?? 'image'
  const stem = base.replace(/\.[^.]+$/, '')
  const filename = stem + '_min' + ext
  const path = await requestSavePath(t('toolbox.imgSaveOne'), filename)
  if (!path) return
  try {
    await ToolboxService.SaveBytes(path, res.data ?? '')
    const where = await publishSaved(path, filename)
    message.success(t('toolbox.imgSavedTo') + (where || path))
  } catch (e) {
    message.error(errText(e))
  }
}

// imgExportZip packs every compressed result into one ZIP archive.
async function imgExportZip() {
  if (imgResults.value.length === 0) {
    message.warning(t('toolbox.imgNeedResult'))
    return
  }
  const path = await requestSavePath(t('toolbox.imgExportZip'), 'images_min.zip')
  if (!path) return
  imgRunning.value = true
  try {
    await ToolboxService.CompressBatchZip(imgPickPaths.value, path, imgBuildOpts())
    const where = await publishSaved(path, 'images_min.zip')
    message.success(t('toolbox.imgExportSaved') + (where || path))
  } catch (e) {
    message.error(errText(e))
  } finally {
    imgRunning.value = false
  }
}

function imgClear() {
  imgPickPaths.value = []
  imgResults.value = []
}

/* ---- image format conversion ---- */
const cvPickPaths = ref<string[]>([])
const cvTarget = ref('png')
const cvQuality = ref<number | null>(null)
const cvMaxDim = ref<number | null>(null)
const cvRunning = ref(false)
const cvResults = ref<ConvertResult[]>([])

const cvTargetOptions = [
  { label: 'PNG', value: 'png' },
  { label: 'JPEG', value: 'jpeg' },
  { label: 'GIF', value: 'gif' },
  { label: 'BMP', value: 'bmp' },
  { label: 'TIFF', value: 'tiff' },
]

function cvExtOf(format: string): string {
  if (format === 'jpeg') return '.jpg'
  if (format === 'tiff') return '.tiff'
  return '.' + format
}

function cvBuildOpts(): ConvertOptions {
  return {
    Target: cvTarget.value,
    Quality: cvQuality.value ?? 0,
    MaxDim: cvMaxDim.value ?? 0,
  }
}

async function cvPick() {
  const picked = await Dialogs.OpenFile({
    Title: t('toolbox.cvPick'),
    AllowsMultipleSelection: true,
    Filters: [{ DisplayName: t('toolbox.cvTab'), Pattern: '*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.webp;*.tif;*.tiff' }],
  })
  const paths = Array.isArray(picked) ? picked : picked ? [picked] : []
  if (paths.length === 0) return
  cvPickPaths.value = paths
  cvResults.value = []
}

async function cvRun() {
  if (cvPickPaths.value.length === 0) {
    message.warning(t('toolbox.cvEmpty'))
    return
  }
  cvRunning.value = true
  try {
    cvResults.value = ((await ToolboxService.ConvertBatch(cvPickPaths.value, '', cvBuildOpts())) ?? []) as ConvertResult[]
    message.success(t('toolbox.cvDone'))
  } catch (e) {
    message.error(errText(e))
  } finally {
    cvRunning.value = false
  }
}

async function cvSaveOne(i: number) {
  const src = cvPickPaths.value[i]
  const res = cvResults.value[i]
  if (!src || !res) return
  const base = src.replace(/\\/g, '/').split('/').pop() ?? 'image'
  const stem = base.replace(/\.[^.]+$/, '')
  const filename = stem + '_conv' + cvExtOf(res.toFormat)
  const path = await requestSavePath(t('toolbox.cvSaveOne'), filename)
  if (!path) return
  try {
    await ToolboxService.SaveBytes(path, res.data ?? '')
    const where = await publishSaved(path, filename)
    message.success(t('toolbox.cvSavedTo') + (where || path))
  } catch (e) {
    message.error(errText(e))
  }
}

async function cvExportZip() {
  if (cvResults.value.length === 0) {
    message.warning(t('toolbox.cvNeedResult'))
    return
  }
  const path = await requestSavePath(t('toolbox.cvExportZip'), 'images_conv.zip')
  if (!path) return
  cvRunning.value = true
  try {
    await ToolboxService.ConvertBatchZip(cvPickPaths.value, path, cvBuildOpts())
    const where = await publishSaved(path, 'images_conv.zip')
    message.success(t('toolbox.cvExportSaved') + (where || path))
  } catch (e) {
    message.error(errText(e))
  } finally {
    cvRunning.value = false
  }
}

function cvClear() {
  cvPickPaths.value = []
  cvResults.value = []
}

/* ---- folder compression ---- */
async function folderZipPick() {
  const picked = await Dialogs.OpenFile({
    Title: t('toolbox.folderZipPick'),
    AllowsMultipleSelection: true,
    CanChooseDirectories: true,
    CanChooseFiles: false,
  })
  folderZipPaths.value = Array.isArray(picked) ? picked : picked ? [picked] : []
  folderZipResult.value = ''
}

async function folderZipRun() {
  if (folderZipPaths.value.length === 0) {
    message.warning(t('toolbox.folderZipEmpty'))
    return
  }
  folderZipRunning.value = true
  try {
    const created = (await ToolboxService.CompressFoldersZip(folderZipPaths.value)) ?? []
    folderZipResult.value = created.join('\n')
    message.success(t('toolbox.folderZipDone', { n: created.length }))
  } catch (e) {
    message.error(errText(e))
  } finally {
    folderZipRunning.value = false
  }
}

function folderZipClear() {
  folderZipPaths.value = []
  folderZipResult.value = ''
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

function pct(n: number): string {
  return Math.round(n * 100) + '%'
}

/* ---- watermark ---- */
const wmPaths = ref<string[]>([])
const wmActive = ref(0)
const wmText = ref('内部资料 请勿外传')
const wmFontSize = ref(48)
const wmFont = ref('sans')
const wmColor = ref('#ff3b30')
const wmOpacity = ref(0.35)
const wmRotation = ref(-30)
const wmDiagonal = ref(false)
// Page rotation is an editing parameter (multiple of 90): preview and export
// both apply it together with the watermark.
const wmPageRotation = ref(0)
const wmTile = ref(false)
const wmDensity = ref(50)
const wmAnchor = ref('center')
const wmOnTop = ref(true)
const wmPages = ref('')
const wmRunning = ref(false)
const wmPreviewUrl = ref('')
const wmPreviewErr = ref('')
const wmInfo = ref('')

const wmFontOptions = [
  { label: t('toolbox.wmFontSans'), value: 'sans' },
  { label: t('toolbox.wmFontSerif'), value: 'serif' },
  { label: t('toolbox.wmFontMono'), value: 'mono' },
]
// Tile density is a percentage (20..300, higher = tighter). A slider lets the
// user pick any value instead of a handful of fixed presets.
const WM_DENSITY_MIN = 20
const WM_DENSITY_MAX = 300
const wmAnchorOptions = [
  { label: t('toolbox.wmAnchorCenter'), value: 'center' },
  { label: t('toolbox.wmAnchorTopLeft'), value: 'tl' },
  { label: t('toolbox.wmAnchorTopCenter'), value: 'tc' },
  { label: t('toolbox.wmAnchorTopRight'), value: 'tr' },
  { label: t('toolbox.wmAnchorLeft'), value: 'l' },
  { label: t('toolbox.wmAnchorRight'), value: 'r' },
  { label: t('toolbox.wmAnchorBottomLeft'), value: 'bl' },
  { label: t('toolbox.wmAnchorBottomCenter'), value: 'bc' },
  { label: t('toolbox.wmAnchorBottomRight'), value: 'br' },
]

function wmIsPdf(p: string): boolean {
  return p.toLowerCase().endsWith('.pdf')
}

function wmBuildOpts(): WatermarkTextOptions {
  return {
    text: wmText.value,
    fontSize: wmFontSize.value,
    font: wmFont.value,
    fontPath: '',
    color: wmColor.value,
    opacity: wmOpacity.value,
    rotation: wmRotation.value,
    diagonal: wmDiagonal.value,
    tile: wmTile.value,
    anchor: wmAnchor.value,
    onTop: wmOnTop.value,
    scale: 0,
    density: wmDensity.value,
    pageRotation: wmPageRotation.value,
  }
}

async function wmPick() {
  const picked = await Dialogs.OpenFile({
    Title: t('toolbox.wmPick'),
    AllowsMultipleSelection: true,
    Filters: [
      { DisplayName: t('toolbox.wmTab'), Pattern: '*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.webp;*.tif;*.tiff;*.pdf' },
      { DisplayName: 'PDF', Pattern: '*.pdf' },
      { DisplayName: t('toolbox.imgTab'), Pattern: '*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.webp;*.tif;*.tiff' },
    ],
  })
  const paths = Array.isArray(picked) ? picked : picked ? [picked] : []
  if (paths.length === 0) return
  wmPaths.value = paths
  wmActive.value = 0
  wmPreviewUrl.value = ''
  wmPreviewErr.value = ''
  wmInfo.value = ''
  await wmPreview()
}

function wmClear() {
  wmPaths.value = []
  wmActive.value = 0
  wmPreviewUrl.value = ''
  wmPreviewErr.value = ''
  wmInfo.value = ''
}

function wmSelectActive(i: number) {
  wmActive.value = i
  void wmPreview()
}

async function wmPreview() {
  const path = wmPaths.value[wmActive.value]
  wmPreviewUrl.value = ''
  wmPreviewErr.value = ''
  wmInfo.value = ''
  if (!path) return
  try {
    if (wmIsPdf(path)) {
      const info = await ToolboxService.PDFPageInfo(path)
      if (info) wmInfo.value = t('toolbox.wmPdfInfo', { n: info.pages })
      const b64 = await ToolboxService.PreviewPDF(path, wmBuildOpts(), 900)
      if (b64) wmPreviewUrl.value = 'data:image/png;base64,' + b64
      else wmPreviewErr.value = t('toolbox.wmNoPreview')
    } else {
      const b64 = await ToolboxService.PreviewImage(path, wmBuildOpts(), 900)
      if (b64) wmPreviewUrl.value = 'data:image/png;base64,' + b64
    }
  } catch (e) {
    wmPreviewErr.value = errText(e)
  }
}

// wmExport writes the watermarked result. A single file is saved via a save
// dialog; multiple files are packed into one ZIP archive.
async function wmExport() {
  if (wmPaths.value.length === 0) {
    message.warning(t('toolbox.wmEmpty'))
    return
  }
  wmRunning.value = true
  try {
    if (wmPaths.value.length === 1) {
      const path = wmPaths.value[0]
      const base = path.replace(/\\/g, '/').split('/').pop() ?? 'file'
      const stem = base.replace(/\.[^.]+$/, '')
      const ext = wmIsPdf(path) ? '.pdf' : base.replace(/^.*(\.[^.]+)$/, '$1')
      const name = stem + '_wm' + ext
      const dest = await requestSavePath(t('toolbox.wmApply'), name)
      if (!dest) return
      if (wmIsPdf(path)) {
        await ToolboxService.WatermarkPDF(path, dest, wmPages.value, wmBuildOpts())
      } else {
        await ToolboxService.WatermarkImage(path, dest, wmBuildOpts())
      }
      const where = await publishSaved(dest, name)
      message.success(t('toolbox.wmDone', { n: 1 }) + savedLocationSuffix(where))
    } else {
      const name = 'watermarked.zip'
      const dest = await requestSavePath(t('toolbox.wmExportZip'), name)
      if (!dest) return
      await ToolboxService.WatermarkBatchZip(wmPaths.value, dest, wmPages.value, wmBuildOpts())
      const where = await publishSaved(dest, name)
      message.success(t('toolbox.wmExportSaved') + (where || dest))
    }
  } catch (e) {
    message.error(errText(e))
  } finally {
    wmRunning.value = false
  }
}

// wmRotate toggles the page-rotation editing parameter (cumulative, multiple
// of 90). The preview refreshes immediately; the rotation is applied together
// with the watermark on export, so no separate _rot file is produced.
async function wmRotate(deg: number) {
  wmPageRotation.value = (((wmPageRotation.value + deg) % 360) + 360) % 360
  if (wmPaths.value.length > 0) await wmPreview()
}

async function wmResetRotation() {
  wmPageRotation.value = 0
  if (wmPaths.value.length > 0) await wmPreview()
}

</script>

<template>
  <div class="page">
    <h3 class="page-title">{{ t('toolbox.title') }}</h3>
    <n-tabs type="line" animated pane-class="tb-pane" pane-style="overflow-y: auto; max-height: calc(100vh - 150px); padding-right: 4px" :theme-overrides="{ tabFontWeight: '500' }">
      <n-tab-pane name="catDaily" :tab="t('toolbox.catDailyTab')">
        <n-tabs class="tb-subtabs" type="segment" animated pane-class="tb-pane" style="margin-top: 4px">
    <n-tab-pane name="imgcmp" :tab="t('toolbox.imgTab')">
            <n-card size="small">
              <n-space vertical>
                <div class="tb-hint">{{ t('toolbox.imgDesc') }}</div>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="imgPick">{{ t('toolbox.imgPick') }}</n-button>
                  <n-button size="small" :disabled="imgPickPaths.length === 0" @click="imgClear">{{ t('toolbox.imgClear') }}</n-button>
                  <n-tag size="small" :bordered="false">{{ imgPickPaths.length }}</n-tag>
                </n-space>
                <n-space align="center">
                  <n-input-number v-model:value="imgTargetKB" :min="0" :max="51200" :placeholder="t('toolbox.imgNoTarget')" style="width: 150px">
                    <template #suffix>KB</template>
                  </n-input-number>
                  <n-input-number v-model:value="imgMaxDim" :min="0" :max="20000" :placeholder="t('toolbox.imgDimAuto')" style="width: 150px" />
                  <n-input-number v-model:value="imgQuality" :min="0" :max="100" :placeholder="t('toolbox.imgQuality')" style="width: 130px" />
                  <n-select v-model:value="imgFormat" :options="imgFormatOptions" style="width: 130px" />
                </n-space>
                <n-space align="center">
                  <n-button size="small" type="primary" :loading="imgRunning" @click="imgRun">{{ t('toolbox.imgRun') }}</n-button>
                </n-space>
                <div class="tb-hint">{{ t('toolbox.imgExportHint') }}</div>
                <div v-if="imgResults.length === 0" class="tb-hint">{{ t('toolbox.imgNoImages') }}</div>
                <div v-else class="imgcmp-table">
                  <div class="imgcmp-row imgcmp-head">
                    <span>{{ t('toolbox.imgResults') }}</span>
                    <span>{{ t('toolbox.imgOrig') }}</span>
                    <span>{{ t('toolbox.imgNew') }}</span>
                    <span>{{ t('toolbox.imgSaveAll') }}</span>
                  </div>
                  <div v-for="(r, i) in imgResults" :key="i" class="imgcmp-row">
                    <span class="imgcmp-name" :title="r.name">{{ r.name }}</span>
                    <span>{{ fmtBytes(r.origSize) }}</span>
                    <span>{{ fmtBytes(r.newSize) }} · {{ pct(r.ratio) }}</span>
                    <n-button size="tiny" @click="imgSaveOne(i)">{{ t('toolbox.imgSaveOne') }}</n-button>
                  </div>
                </div>
                <n-space v-if="imgResults.length > 0" align="center">
                  <n-button size="small" type="primary" :loading="imgRunning" @click="imgExportZip">{{ t('toolbox.imgExportZip') }}</n-button>
                </n-space>
                <n-divider style="margin: 4px 0" />
                <div class="tb-hint">{{ t('toolbox.folderZipDesc') }}</div>
                <n-space align="center">
                  <n-button size="small" @click="folderZipPick">{{ t('toolbox.folderZipPick') }}</n-button>
                  <n-button size="small" :disabled="folderZipPaths.length === 0" @click="folderZipClear">{{ t('toolbox.imgClear') }}</n-button>
                  <n-tag size="small" :bordered="false">{{ folderZipPaths.length }}</n-tag>
                  <n-button size="small" type="primary" :loading="folderZipRunning" :disabled="folderZipPaths.length === 0" @click="folderZipRun">{{ t('toolbox.folderZipRun') }}</n-button>
                </n-space>
                <pre v-if="folderZipResult" class="tb-hint" style="white-space: pre-wrap; margin: 0">{{ t('toolbox.folderZipSaved') }}{{ '\n' }}{{ folderZipResult }}</pre>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="imgconv" :tab="t('toolbox.cvTab')">
            <n-card size="small">
              <n-space vertical>
                <div class="tb-hint">{{ t('toolbox.cvDesc') }}</div>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="cvPick">{{ t('toolbox.cvPick') }}</n-button>
                  <n-button size="small" :disabled="cvPickPaths.length === 0" @click="cvClear">{{ t('toolbox.cvClear') }}</n-button>
                  <n-tag size="small" :bordered="false">{{ cvPickPaths.length }}</n-tag>
                </n-space>
                <n-space align="center">
                  <span class="txt-label">{{ t('toolbox.cvTarget') }}</span>
                  <n-select v-model:value="cvTarget" :options="cvTargetOptions" style="width: 130px" />
                  <n-input-number v-model:value="cvQuality" :min="0" :max="100" :placeholder="t('toolbox.cvQuality')" style="width: 140px" />
                  <n-input-number v-model:value="cvMaxDim" :min="0" :max="20000" :placeholder="t('toolbox.cvDimAuto')" style="width: 150px" />
                </n-space>
                <n-space align="center">
                  <n-button size="small" type="primary" :loading="cvRunning" @click="cvRun">{{ t('toolbox.cvRun') }}</n-button>
                </n-space>
                <div class="tb-hint">{{ t('toolbox.cvExportHint') }}</div>
                <div v-if="cvResults.length === 0" class="tb-hint">{{ t('toolbox.cvNoImages') }}</div>
                <div v-else class="imgcmp-table">
                  <div class="imgcmp-row imgcmp-head">
                    <span>{{ t('toolbox.cvResults') }}</span>
                    <span>{{ t('toolbox.cvFromTo') }}</span>
                    <span>{{ t('toolbox.cvSize') }}</span>
                    <span>{{ t('toolbox.cvSaveAll') }}</span>
                  </div>
                  <div v-for="(r, i) in cvResults" :key="i" class="imgcmp-row">
                    <span class="imgcmp-name" :title="r.name">{{ r.name }}</span>
                    <span>{{ (r.fromExt || '?').toUpperCase() }} → {{ r.toFormat.toUpperCase() }}</span>
                    <span>{{ r.width }}×{{ r.height }} · {{ fmtBytes(r.size) }}</span>
                    <n-button size="tiny" @click="cvSaveOne(i)">{{ t('toolbox.cvSaveOne') }}</n-button>
                  </div>
                </div>
                <n-space v-if="cvResults.length > 0" align="center">
                  <n-button size="small" type="primary" :loading="cvRunning" @click="cvExportZip">{{ t('toolbox.cvExportZip') }}</n-button>
                </n-space>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="watermark" :tab="t('toolbox.wmTab')">
            <n-card size="small">
              <n-space vertical>
                <div class="tb-hint">{{ t('toolbox.wmDesc') }}</div>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="wmPick">{{ t('toolbox.wmPick') }}</n-button>
                  <n-button size="small" :disabled="wmPaths.length === 0" @click="wmClear">{{ t('toolbox.wmClear') }}</n-button>
                  <n-tag size="small" :bordered="false">{{ t('toolbox.wmFileCount', { n: wmPaths.length }) }}</n-tag>
                  <n-tag v-if="wmInfo" size="small" type="info" :bordered="false">{{ wmInfo }}</n-tag>
                </n-space>
                <n-space v-if="wmPaths.length > 0" align="center" :wrap="true">
                  <n-tag
                    v-for="(p, i) in wmPaths"
                    :key="i"
                    size="small"
                    :bordered="false"
                    :type="i === wmActive ? 'primary' : 'default'"
                    style="cursor: pointer; max-width: 280px"
                    :title="p"
                    @click="wmSelectActive(i)"
                  >
                    {{ p.split('/').pop() }}
                  </n-tag>
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.wmText') }}</span>
                  <n-input v-model:value="wmText" size="small" :placeholder="t('toolbox.wmTextPlaceholder')" style="width: 240px" />
                  <span class="txt-label">{{ t('toolbox.wmFontSize') }}</span>
                  <n-input-number v-model:value="wmFontSize" :min="6" :max="400" size="small" style="width: 110px" />
                  <n-select v-model:value="wmFont" :options="wmFontOptions" size="small" style="width: 120px" />
                  <span class="txt-label">{{ t('toolbox.wmColor') }}</span>
                  <n-color-picker v-model:value="wmColor" size="small" :show-alpha="false" style="width: 90px" />
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.wmOpacity') }}</span>
                  <n-slider v-model:value="wmOpacity" :min="0.05" :max="1" :step="0.05" style="width: 150px" />
                  <span class="txt-label">{{ t('toolbox.wmRotation') }}</span>
                  <n-slider v-model:value="wmRotation" :min="-180" :max="180" :step="1" :disabled="wmDiagonal" style="width: 150px" />
                  <span class="txt-label">{{ wmRotation }}°</span>
                  <n-checkbox v-model:checked="wmDiagonal">{{ t('toolbox.wmDiagonal') }}</n-checkbox>
                  <n-checkbox v-model:checked="wmTile">{{ t('toolbox.wmTile') }}</n-checkbox>
                  <span class="txt-label">{{ t('toolbox.wmDensity') }}</span>
                  <n-slider
                    v-model:value="wmDensity"
                    :min="WM_DENSITY_MIN"
                    :max="WM_DENSITY_MAX"
                    :step="1"
                    :disabled="!wmTile"
                    style="width: 150px"
                  />
                  <n-input-number
                    v-model:value="wmDensity"
                    :min="WM_DENSITY_MIN"
                    :max="WM_DENSITY_MAX"
                    size="small"
                    style="width: 110px"
                    :disabled="!wmTile"
                  >
                    <template #suffix>%</template>
                  </n-input-number>
                  <span class="tb-hint">{{ t('toolbox.wmDensityHint') }}</span>
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.wmAnchor') }}</span>
                  <n-select v-model:value="wmAnchor" :options="wmAnchorOptions" size="small" style="width: 130px" />
                  <n-checkbox v-model:checked="wmOnTop">{{ t('toolbox.wmOnTop') }}</n-checkbox>
                  <span class="txt-label">{{ t('toolbox.wmPages') }}</span>
                  <n-input v-model:value="wmPages" size="small" :placeholder="t('toolbox.wmPagesPlaceholder')" style="width: 220px" />
                </n-space>
                <div class="tb-hint">{{ t('toolbox.wmExportHint') }}</div>
                <n-space align="center" :wrap="true">
                  <n-button size="small" type="primary" :loading="wmRunning" :disabled="wmPaths.length === 0" @click="wmExport">
                    {{ wmPaths.length > 1 ? t('toolbox.wmExportZip') : t('toolbox.wmApply') }}
                  </n-button>
                  <n-button size="small" @click="wmPreview" :disabled="wmPaths.length === 0">{{ t('toolbox.wmPreview') }}</n-button>
                  <n-divider vertical />
                  <span class="txt-label">{{ t('toolbox.wmPageRotate') }}</span>
                  <n-button size="small" @click="wmRotate(90)">{{ t('toolbox.wmRot90') }}</n-button>
                  <n-button size="small" @click="wmRotate(180)">{{ t('toolbox.wmRot180') }}</n-button>
                  <n-button size="small" @click="wmRotate(270)">{{ t('toolbox.wmRot270') }}</n-button>
                  <n-button size="small" :disabled="wmPageRotation === 0" @click="wmResetRotation">{{ t('toolbox.wmRotReset') }}</n-button>
                  <n-tag size="small" :bordered="false" :type="wmPageRotation !== 0 ? 'primary' : 'default'">{{ wmPageRotation }}°</n-tag>
                </n-space>
                <div class="wm-preview">
                  <img v-if="wmPreviewUrl" :src="wmPreviewUrl" alt="preview" />
                  <div v-else class="tb-hint">{{ wmPreviewErr || t('toolbox.wmPreviewHint') }}</div>
                </div>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="pwgen" :tab="t('toolbox.pwgenTab')">
            <n-card size="small">
              <n-space vertical size="large">
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.pwdComplexity') }}</span>
                  <n-select v-model:value="pwLevel" :options="pwLevelOptions" size="small" style="width: 190px" />
                  <span class="txt-label">{{ t('toolbox.pwdLength') }}</span>
                  <n-input-number v-model:value="pwLength" :min="4" :max="128" size="small" style="width: 110px" />
                  <n-checkbox v-model:checked="pwNoAmbiguous" size="small">{{ t('toolbox.pwExcludeAmbiguous') }}</n-checkbox>
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:14px">
                  <n-checkbox :checked="pwUseLower === null ? undefined : pwUseLower" @update:checked="(v: boolean) => (pwUseLower = v)" size="small">
                    {{ t('toolbox.pwLower') }} <span class="pw-dim">{{ t('toolbox.pwFollowLevel') }}</span>
                  </n-checkbox>
                  <n-checkbox :checked="pwUseUpper === null ? undefined : pwUseUpper" @update:checked="(v: boolean) => (pwUseUpper = v)" size="small">
                    {{ t('toolbox.pwUpper') }} <span class="pw-dim">{{ t('toolbox.pwFollowLevel') }}</span>
                  </n-checkbox>
                  <n-checkbox :checked="pwUseDigits === null ? undefined : pwUseDigits" @update:checked="(v: boolean) => (pwUseDigits = v)" size="small">
                    {{ t('toolbox.digits') }} <span class="pw-dim">{{ t('toolbox.pwFollowLevel') }}</span>
                  </n-checkbox>
                  <n-checkbox :checked="pwUseSymbols === null ? undefined : pwUseSymbols" @update:checked="(v: boolean) => (pwUseSymbols = v)" size="small">
                    {{ t('toolbox.pwSymbolsToggle') }} <span class="pw-dim">{{ t('toolbox.pwFollowLevel') }}</span>
                  </n-checkbox>
                  <n-button size="tiny" quaternary @click="resetPwOverrides">{{ t('toolbox.pwResetFollow') }}</n-button>
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.pwCustomSymbolsLabel') }}</span>
                  <n-input v-model:value="pwCustomSymbols" size="small" :placeholder="t('toolbox.pwCustomSymbolsHint')" style="width: 260px" />
                </n-space>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="doGeneratePassword">{{ t('toolbox.pwGenerateBtn') }}</n-button>
                  <n-button size="small" quaternary @click="copy(pwOutput)">{{ t('toolbox.copy') }}</n-button>
                  <n-button size="small" @click="openPwSave">{{ t('toolbox.pwSaveVaultBtn') }}</n-button>
                </n-space>
                <n-input v-model:value="pwOutput" readonly :placeholder="t('toolbox.pwOutputPlaceholder')" class="pw-output" />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="time" :tab="t('toolbox.timeTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="tsInput" :placeholder="t('toolbox.unixTs')" style="width: 200px" />
                  <n-select v-model:value="tsUnit" :options="[{ label: t('toolbox.seconds'), value: 's' }, { label: t('toolbox.milliseconds'), value: 'ms' }]" style="width: 90px" />
                  <n-button size="small" type="primary" @click="tsToTime">{{ t('toolbox.toRFC3339') }}</n-button>
                  <n-input v-model:value="tsOutput" readonly :placeholder="t('toolbox.result')" style="width: 320px" />
                </n-space>
                <n-space align="center">
                  <n-input v-model:value="timeInput" placeholder="2024-01-01T00:00:00Z" style="width: 320px" />
                  <n-button size="small" type="primary" @click="timeToTs">{{ t('toolbox.toTimestamp') }}</n-button>
                  <n-input v-model:value="timeOutput" readonly :placeholder="t('toolbox.result')" style="width: 200px" />
                </n-space>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="timediff" :tab="t('toolbox.timeDiffTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="tdFrom" :placeholder="t('toolbox.timeDiffFrom')" style="width: 300px" />
                  <n-input v-model:value="tdTo" :placeholder="t('toolbox.timeDiffTo')" style="width: 300px" />
                  <n-button size="small" @click="tdFillNow">{{ t('toolbox.timeDiffNow') }}</n-button>
                  <n-button size="small" type="primary" @click="calcTimeDiff">{{ t('toolbox.calc') }}</n-button>
                </n-space>
                <template v-if="tdResult">
                  <n-space align="center">
                    <n-tag size="small" :type="tdResult.seconds >= 0 ? 'success' : 'warning'">{{ tdResult.seconds }} s</n-tag>
                    <span class="tb-mono">{{ tdResult.humanized }}</span>
                    <span class="tb-mono">({{ tdResult.absHumanized }})</span>
                  </n-space>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="qr" :tab="t('toolbox.qrTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="qrText" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.qrPlaceholder')" style="width: 420px" />
                  <n-select v-model:value="qrLevel" :options="[{ label: 'L', value: 'L' }, { label: 'M', value: 'M' }, { label: 'Q', value: 'Q' }, { label: 'H', value: 'H' }]" style="width: 90px" />
                  <n-input-number v-model:value="qrScale" :min="2" :max="20" style="width: 120px" />
                  <n-button size="small" type="primary" @click="genQR">{{ t('toolbox.generate') }}</n-button>
                </n-space>
                <div v-if="qrImage" style="text-align: center">
                  <img :src="qrImage" alt="QR" style="max-width: 320px" />
                  <div style="margin-top: 8px">
                    <n-space justify="center">
                      <n-button size="small" @click="downloadQR">{{ t('toolbox.download') }}</n-button>
                      <n-button size="small" @click="copy(qrText)">{{ t('toolbox.copy') }}</n-button>
                    </n-space>
                  </div>
                </div>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="idlookup" :tab="t('toolbox.idlookupTab')">
            <n-card size="small">
              <n-space vertical>
                <n-divider title-placement="left" style="margin: 0">{{ t('toolbox.phoneSection') }}</n-divider>
                <n-space align="center">
                  <n-input v-model:value="phInput" :placeholder="t('toolbox.phonePlaceholder')" style="width: 260px" @keyup.enter="lookupPhone" />
                  <n-button size="small" type="primary" :loading="phLoading" @click="lookupPhone">{{ t('toolbox.lookup') }}</n-button>
                </n-space>
                <n-alert v-if="phResult && !phResult.ok" type="warning" :show-icon="true">{{ phResult.message || t('toolbox.phoneNotFound') }}</n-alert>
                <n-descriptions v-if="phResult && phResult.ok" :column="2" bordered size="small" label-placement="left" style="max-width: 640px">
                  <n-descriptions-item :label="t('toolbox.phoneNumber')">{{ phResult.input }}</n-descriptions-item>
                  <n-descriptions-item :label="t('toolbox.phoneOperator')">{{ phResult.operator }}</n-descriptions-item>
                  <n-descriptions-item :label="t('toolbox.phoneProvince')">{{ phResult.province || '-' }}</n-descriptions-item>
                  <n-descriptions-item :label="t('toolbox.phoneCity')">{{ phResult.city || '-' }}</n-descriptions-item>
                  <n-descriptions-item :label="t('toolbox.phoneAreaCode')">{{ phResult.areaCode || '-' }}</n-descriptions-item>
                  <n-descriptions-item :label="t('toolbox.phoneZip')">{{ phResult.zipCode || '-' }}</n-descriptions-item>
                </n-descriptions>
                <n-text v-if="phResult && phResult.ok" depth="3" style="font-size: 12px">
                  {{ t('toolbox.phoneDataVersion') }}: {{ phResult.dataVersion }}
                </n-text>

                <n-divider title-placement="left" style="margin: 12px 0 0">{{ t('toolbox.idSection') }}</n-divider>
                <n-space align="center">
                  <n-input v-model:value="idInput" :placeholder="t('toolbox.idPlaceholder')" style="width: 320px" @keyup.enter="lookupIDCard" />
                  <n-button size="small" type="primary" :loading="idLoading" @click="lookupIDCard">{{ t('toolbox.lookup') }}</n-button>
                </n-space>
                <template v-if="idResult">
                  <n-space align="center">
                    <n-tag :type="idResult.valid ? 'success' : 'error'" size="small">
                      {{ idResult.valid ? t('toolbox.idValid') : t('toolbox.idInvalid') }}
                    </n-tag>
                    <n-text depth="3" style="font-size: 12px">{{ t('toolbox.idOfflineHint') }}</n-text>
                  </n-space>
                  <n-alert v-if="idResult.reasons && idResult.reasons.length" type="error" :show-icon="true">
                    <div v-for="(r, i) in idResult.reasons" :key="i">· {{ r }}</div>
                  </n-alert>
                  <n-descriptions :column="2" bordered size="small" label-placement="left" style="max-width: 640px">
                    <n-descriptions-item :label="t('toolbox.phoneNumber')">{{ idResult.input }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.idChecksum')">
                      {{ idResult.checksumOk ? '✓' : '✗' }}
                    </n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.phoneProvince')">{{ idResult.province || '-' }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.phoneCity')">{{ idResult.city || '-' }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.idDistrict')">{{ idResult.district || '-' }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.idBirthday')">{{ idResult.birthday || '-' }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.idGender')">{{ idResult.gender || '-' }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.idLength')">{{ idResult.length }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="catText" :tab="t('toolbox.catTextTab')">
        <n-tabs class="tb-subtabs" type="segment" animated pane-class="tb-pane" style="margin-top: 4px">
    <n-tab-pane name="text" :tab="t('toolbox.textTab')">
            <n-card size="small">
              <n-space vertical size="large">
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.separator') }}</span>
                  <n-select v-model:value="txtSep" :options="sepOptions" size="small" style="width: 110px" />
                  <n-input v-if="txtSep === 'custom'" v-model:value="txtSepCustom" size="small" :placeholder="t('toolbox.custom')" style="width: 90px" />
                  <span class="txt-label">{{ t('toolbox.quoteLabel') }}</span>
                  <n-select v-model:value="txtQuote" :options="quoteOptions" size="small" style="width: 90px" />
                  <n-checkbox v-model:checked="txtSortDedupe" size="small">{{ t('toolbox.sortAfterDedupe') }}</n-checkbox>
                  <n-checkbox v-model:checked="txtNoHeader" size="small">{{ t('toolbox.noHeader') }}</n-checkbox>
                </n-space>

                <n-space vertical size="small">
                  <div class="txt-label">{{ t('toolbox.affixHint') }}</div>
                  <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                    <n-input v-model:value="affixPrefix" size="small" :placeholder="t('toolbox.pwdPrefix')" style="width: 140px" />
                    <span class="txt-label">+</span>
                    <span class="txt-label">{{ t('toolbox.eachLine') }}</span>
                    <span class="txt-label">+</span>
                    <n-input v-model:value="affixSuffix" size="small" :placeholder="t('toolbox.pwdSuffix')" style="width: 140px" />
                    <n-button size="small" type="primary" @click="doAffixJoin">{{ t('toolbox.join') }}</n-button>
                  </n-space>
                </n-space>

                <n-space vertical size="small">
                  <div class="txt-label">{{ t('toolbox.input') }}</div>
                  <n-input v-model:value="txtInput" type="textarea" :rows="8" :placeholder="t('toolbox.inputPlaceholder')" />
                </n-space>

                <n-space :wrap="true">
                  <n-button size="small" type="primary" @click="doTranspose('r2c')">{{ t('toolbox.transpose') }}</n-button>
                  <n-button size="small" type="primary" @click="doTranspose('c2r')">{{ t('toolbox.columnize') }}</n-button>
                  <n-button size="small" @click="doJoin">{{ t('toolbox.join') }}</n-button>
                  <n-button size="small" @click="doQuote">{{ t('toolbox.quote') }}</n-button>
                  <n-button size="small" @click="doDedupe">{{ t('toolbox.dedupe') }}</n-button>
                  <n-button size="small" @click="doTextToJSON">{{ t('toolbox.textToJSON') }}</n-button>
                  <n-button size="small" @click="doJSONToText">{{ t('toolbox.jsonToText') }}</n-button>
                </n-space>

                <n-space vertical size="small">
                  <n-space align="center" justify="space-between" style="width: 100%">
                    <div class="txt-label">{{ t('toolbox.output') }}</div>
                    <n-button size="tiny" quaternary @click="useOutputAsInput">{{ t('toolbox.outputAsInput') }}</n-button>
                  </n-space>
                  <n-input v-model:value="txtOutput" type="textarea" :rows="8" readonly :placeholder="t('toolbox.result')" />
                  <n-button size="small" quaternary @click="copy(txtOutput)">{{ t('toolbox.copyResult') }}</n-button>
                </n-space>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="enc" :tab="t('toolbox.encTab')">
            <n-card size="small">
              <n-space vertical>
                <n-input v-model:value="encInput" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.inputContent')" />
                <n-space>
                  <n-button size="small" @click="b64Encode">{{ t('toolbox.base64Encode') }}</n-button>
                  <n-button size="small" @click="b64Decode">{{ t('toolbox.base64Decode') }}</n-button>
                  <n-button size="small" @click="hexEncode">{{ t('toolbox.hexEncode') }}</n-button>
                  <n-button size="small" @click="hexDecode">{{ t('toolbox.hexDecode') }}</n-button>
                  <n-button size="small" quaternary @click="copy(encOutput)">{{ t('toolbox.copyResult') }}</n-button>
                </n-space>
                <n-input v-model:value="encOutput" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.result')" readonly />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="hash" :tab="t('toolbox.hashTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space>
                  <n-select v-model:value="hashAlgo" :options="hashAlgos" style="width: 140px" />
                  <n-button size="small" type="primary" @click="doHash">{{ t('toolbox.calculate') }}</n-button>
                  <n-button size="small" quaternary @click="copy(hashOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-input v-model:value="hashInput" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.inputContent')" />
                <n-input v-model:value="hashOutput" readonly :placeholder="t('toolbox.digestResult')" />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="json" :tab="t('toolbox.jsonTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space>
                  <n-button size="small" @click="jsonFormat(true)">{{ t('toolbox.format') }}</n-button>
                  <n-button size="small" @click="jsonFormat(false)">{{ t('toolbox.compress') }}</n-button>
                  <n-button size="small" quaternary @click="copy(jsonOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-grid :cols="2" :x-gap="12">
                  <n-gi><n-input v-model:value="jsonInput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.inputJson')" /></n-gi>
                  <n-gi><n-input v-model:value="jsonOutput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.result')" readonly /></n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="yaml" :tab="t('toolbox.yamlTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="yamlFormat">{{ t('toolbox.yamlFormat') }}</n-button>
                  <n-button size="small" @click="yamlValidate">{{ t('toolbox.yamlValidate') }}</n-button>
                  <n-button size="small" @click="yamlToJSON">{{ t('toolbox.yamlToJson') }}</n-button>
                  <n-button size="small" @click="jsonToYAML">{{ t('toolbox.jsonToYaml') }}</n-button>
                  <n-button size="small" quaternary @click="copy(yamlOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-grid :cols="2" :x-gap="12">
                  <n-gi><n-input v-model:value="yamlInput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.yamlPlaceholder')" /></n-gi>
                  <n-gi><n-input v-model:value="yamlOutput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.result')" readonly /></n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="regex" :tab="t('toolbox.regexTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="rePattern" :placeholder="t('toolbox.regexPattern')" style="width: 340px" />
                  <n-checkbox v-model:checked="reIgnoreCase">i</n-checkbox>
                  <n-button size="small" type="primary" @click="runRegex">{{ t('toolbox.regexRun') }}</n-button>
                </n-space>
                <n-input v-model:value="reInput" type="textarea" :autosize="{ minRows: 5 }" :placeholder="t('toolbox.regexInput')" />
                <template v-if="reResult">
                  <n-tag size="small">{{ t('toolbox.regexMatches') }}: {{ reResult.count }}</n-tag>
                  <div v-for="m in reResult.matches" :key="m.index" class="tb-mono">
                    #{{ m.index }} {{ m.text }} <span v-if="m.groups.length > 1">→ {{ m.groups.slice(1).join(', ') }}</span>
                  </div>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="diff" :tab="t('toolbox.diffTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space>
                  <n-button size="small" type="primary" @click="runDiff">{{ t('toolbox.diffRun') }}</n-button>
                  <n-tag v-if="diffResult" size="small" type="success">+{{ diffResult.added }}</n-tag>
                  <n-tag v-if="diffResult" size="small" type="error">-{{ diffResult.deleted }}</n-tag>
                </n-space>
                <n-grid :cols="2" :x-gap="12">
                  <n-gi><n-input v-model:value="diffA" type="textarea" :autosize="{ minRows: 8 }" :placeholder="t('toolbox.diffOriginal')" /></n-gi>
                  <n-gi><n-input v-model:value="diffB" type="textarea" :autosize="{ minRows: 8 }" :placeholder="t('toolbox.diffModified')" /></n-gi>
                </n-grid>
                <template v-if="diffResult">
                  <div v-for="(l, i) in diffResult.lines" :key="i" class="tb-mono tb-diff" :class="'tb-diff-' + l.type">
                    <span class="tb-diff-sign">{{ l.type === 'add' ? '+' : l.type === 'del' ? '-' : ' ' }}</span>{{ l.text }}
                  </div>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="csvsql" :tab="t('toolbox.csvSqlTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="csvTable" :placeholder="t('toolbox.csvTable')" style="width: 180px" />
                  <n-select v-model:value="csvDialect" :options="[{ label: 'PostgreSQL', value: 'postgres' }, { label: 'MySQL', value: 'mysql' }]" style="width: 140px" />
                  <n-checkbox v-model:checked="csvCreate">{{ t('toolbox.csvCreateTable') }}</n-checkbox>
                  <n-button size="small" type="primary" @click="csvToSQL">{{ t('toolbox.csvConvert') }}</n-button>
                  <n-button size="small" quaternary @click="copy(csvOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-grid :cols="2" :x-gap="12">
                  <n-gi><n-input v-model:value="csvInput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.csvPlaceholder')" /></n-gi>
                  <n-gi><n-input v-model:value="csvOutput" type="textarea" :autosize="{ minRows: 10 }" :placeholder="t('toolbox.result')" readonly /></n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="catOps" :tab="t('toolbox.catOpsTab')">
        <n-tabs class="tb-subtabs" type="segment" animated pane-class="tb-pane" style="margin-top: 4px">
    <n-tab-pane name="ssl" :tab="t('toolbox.sslTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="sslHost" :placeholder="t('toolbox.domainExample')" style="width: 280px" @keyup.enter="doSSLCheck" />
                  <n-input-number v-model:value="sslPort" :min="1" :max="65535" style="width: 110px" />
                  <n-input-number v-model:value="sslTimeout" :min="1000" :max="30000" :step="1000" style="width: 130px" />
                  <n-button size="small" type="primary" :loading="sslChecking" @click="doSSLCheck">{{ t('toolbox.detect') }}</n-button>
                </n-space>

                <template v-if="sslResult">
                  <!-- 评级总览 -->
                  <div class="ssl-summary">
                    <div class="ssl-grade" :style="{ color: gradeColor(sslResult.grade) }">
                      <div class="ssl-grade-letter">{{ sslResult.grade }}</div>
                      <div class="ssl-grade-score">{{ t('toolbox.score', { n: sslResult.score }) }}</div>
                    </div>
                    <div class="ssl-meta">
                      <div class="ssl-meta-row"><span class="ssl-label">{{ t('toolbox.sslHost') }}</span>{{ sslResult.host }}:{{ sslResult.port }} <span class="ssl-dim">({{ sslResult.ip }})</span></div>
                      <div class="ssl-meta-row"><span class="ssl-label">{{ t('toolbox.sslNegotiatedVersion') }}</span>{{ sslResult.tlsVersion }}</div>
                      <div class="ssl-meta-row"><span class="ssl-label">{{ t('toolbox.sslNegotiatedCipher') }}</span>{{ sslResult.cipherSuite }}</div>
                      <div class="ssl-meta-row"><span class="ssl-label">{{ t('toolbox.sslHandshakeTime') }}</span>{{ sslResult.latency }}</div>
                      <div class="ssl-meta-row">
                        <span class="ssl-label">{{ t('toolbox.sslRemaining') }}</span>
                        <n-tag size="small" :type="sslResult.expired ? 'error' : sslResult.expiringSoon ? 'warning' : 'success'">
                          {{ sslResult.expired ? t('toolbox.expiredDays', { n: -sslResult.daysRemaining }) : t('toolbox.daysUnit', { n: sslResult.daysRemaining }) }}
                        </n-tag>
                      </div>
                    </div>
                  </div>

                  <!-- 问题列表 -->
                  <div v-if="sslResult.issues.length" class="ssl-issues">
                    <div v-for="(issue, i) in sslResult.issues" :key="i" class="ssl-issue">⚠ {{ sslIssueText(issue) }}</div>
                  </div>
                  <div v-else class="ssl-ok">{{ t('toolbox.sslNoIssuesFound') }}</div>

                  <!-- 协议支持 -->
                  <div class="ssl-section-title">{{ t('toolbox.sslProtoTitle') }}</div>
                  <div class="ssl-protocol-table">
                    <div v-for="p in sslResult.protocols" :key="p.version" class="ssl-protocol-row">
                      <span class="ssl-proto-name">{{ p.version }}</span>
                      <n-tag size="small" :type="p.supported ? (p.version === 'SSL 3.0' || p.version === 'TLS 1.0' || p.version === 'TLS 1.1' ? 'warning' : 'success') : 'default'">
                        {{ p.supported ? t('toolbox.sslSupported') : t('toolbox.sslUnsupported') }}
                      </n-tag>
                      <span class="ssl-dim ssl-cipher">{{ p.cipherName || p.error || '' }}</span>
                    </div>
                  </div>

                  <!-- 证书链 -->
                  <div class="ssl-section-title">{{ t('toolbox.sslChainTitle', { n: sslResult.chain.length }) }}</div>
                  <div class="ssl-chain">
                    <div v-for="(c, i) in sslResult.chain" :key="i" class="ssl-cert">
                      <div class="ssl-cert-head">
                        <span class="ssl-cert-badge">{{ i === 0 ? t('toolbox.sslLeaf') : i === sslResult.chain.length - 1 ? t('toolbox.sslRoot') : t('toolbox.intermediateN', { n: i }) }}</span>
                        <span class="ssl-cert-name">{{ certName(c) }}</span>
                        <n-tag v-if="c.selfSigned" size="tiny" type="warning">{{ t('toolbox.sslSelfSigned') }}</n-tag>
                      </div>
                      <div class="ssl-cert-grid">
                        <div><span class="ssl-label">{{ t('toolbox.issuer') }}</span>{{ issuerName(c) }}</div>
                        <div><span class="ssl-label">{{ t('toolbox.validity') }}</span>{{ c.notBefore }} ~ {{ c.notAfter }}</div>
                        <div><span class="ssl-label">{{ t('toolbox.signatureAlgorithm') }}</span>{{ c.signatureAlgorithm }}</div>
                        <div><span class="ssl-label">{{ t('toolbox.publicKey') }}</span>{{ c.publicKeyAlgorithm }} {{ t('toolbox.keyBits', { n: c.publicKeyBits }) }}</div>
                        <div><span class="ssl-label">{{ t('toolbox.serialNumber') }}</span>{{ c.serialNumber }}</div>
                        <div v-if="i === 0 && c.dnsNames.length"><span class="ssl-label">{{ t('toolbox.sanDomains') }}</span>{{ t('toolbox.sanCount', { n: c.dnsNames.length }) }}</div>
                      </div>
                      <div v-if="i === 0 && c.dnsNames.length" class="ssl-san-list">
                        <n-tag v-for="d in c.dnsNames.slice(0, 12)" :key="d" size="tiny" :bordered="false">{{ d }}</n-tag>
                        <span v-if="c.dnsNames.length > 12" class="ssl-dim">{{ t('toolbox.sanTotal', { n: c.dnsNames.length }) }}</span>
                      </div>
                    </div>
                  </div>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="scan" :tab="t('toolbox.scanTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="scanHost" :placeholder="t('toolbox.host')" style="width: 200px" />
                  <n-input v-model:value="scanPorts" :placeholder="t('toolbox.portsComma')" style="width: 300px" />
                  <n-input-number v-model:value="scanTimeout" :min="100" :max="10000" :step="100" style="width: 140px" />
                  <n-button size="small" @click="expandPortSpecText">{{ t('toolbox.expandPorts') }}</n-button>
                  <n-button size="small" type="primary" :loading="scanning" @click="doScan">{{ t('toolbox.probe') }}</n-button>
                </n-space>
                <n-space v-if="portPresets.length" align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.portPresets') }}</span>
                  <n-button v-for="p in portPresets" :key="p.name" size="tiny" quaternary @click="applyPreset(p.spec)">
                    {{ presetLabel(p.name) }} ({{ p.count }})
                  </n-button>
                </n-space>
                <n-space v-if="scanResults.length" :wrap="true">
                  <n-tag v-for="r in scanResults" :key="r.port" :type="r.open ? 'success' : 'error'" size="small">
                    {{ r.port }} {{ r.open ? t('toolbox.open') : t('toolbox.closed') }} · {{ r.latency }}
                  </n-tag>
                </n-space>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="http" :tab="t('toolbox.httpTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-select v-model:value="hreqMethod" :options="hreqMethods" size="small" style="width: 110px" />
                  <n-input v-model:value="hreqURL" placeholder="https://..." style="width: 420px" />
                  <n-button size="small" type="primary" :loading="hreqRunning" @click="doHTTPRequest">{{ t('toolbox.request') }}</n-button>
                </n-space>
                <n-grid :cols="2" :x-gap="12" :y-gap="8">
                  <n-gi>
                    <div class="txt-label">{{ t('toolbox.headersLabel') }}</div>
                    <n-input v-model:value="hreqHeaders" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.headersPlaceholder')" />
                  </n-gi>
                  <n-gi>
                    <div class="txt-label">{{ t('toolbox.bodyLabel') }}</div>
                    <n-input v-model:value="hreqBody" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.bodyPlaceholder')" />
                  </n-gi>
                </n-grid>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.timeoutLabel') }}</span>
                  <n-input-number v-model:value="hreqTimeout" :min="100" :max="120000" :step="1000" size="small" style="width: 130px" />
                  <span class="txt-label">{{ t('toolbox.retriesLabel') }}</span>
                  <n-input-number v-model:value="hreqRetries" :min="0" :max="10" size="small" style="width: 100px" />
                  <span class="txt-label">{{ t('toolbox.retryDelayLabel') }}</span>
                  <n-input-number v-model:value="hreqRetryDelay" :min="0" :max="10000" :step="100" size="small" style="width: 120px" />
                  <span class="txt-label">{{ t('toolbox.maxBodyLabel') }}</span>
                  <n-input-number v-model:value="hreqMaxBody" :min="0" :max="1048576" :step="1024" size="small" style="width: 130px" />
                  <n-checkbox v-model:checked="hreqFollow" size="small">{{ t('toolbox.followRedirects') }}</n-checkbox>
                </n-space>
                <template v-if="hreqResult">
                  <n-space align="center" :wrap="true">
                    <n-tag :type="hreqResult.status < 400 ? 'success' : 'error'" size="small">
                      {{ hreqResult.status }} {{ hreqResult.statusText }}
                    </n-tag>
                    <n-tag size="small" :bordered="false">{{ hreqResult.proto }}</n-tag>
                    <span style="opacity: 0.7">{{ t('toolbox.elapsed') }} {{ hreqResult.latency }}</span>
                    <span v-if="hreqResult.attempts > 1" style="opacity: 0.7">{{ t('toolbox.attemptsN', { n: hreqResult.attempts }) }}</span>
                    <span style="opacity: 0.7">{{ t('toolbox.bodySizeN', { n: hreqResult.bodySize }) }}</span>
                  </n-space>
                  <n-grid :cols="5" :x-gap="8" :y-gap="4">
                    <n-gi><span class="txt-label">DNS</span> {{ hreqResult.timings.dnsMs }} ms</n-gi>
                    <n-gi><span class="txt-label">TCP</span> {{ hreqResult.timings.connMs }} ms</n-gi>
                    <n-gi><span class="txt-label">TLS</span> {{ hreqResult.timings.tlsMs }} ms</n-gi>
                    <n-gi><span class="txt-label">TTFB</span> {{ hreqResult.timings.ttfbMs }} ms</n-gi>
                    <n-gi><span class="txt-label">Total</span> {{ hreqResult.timings.totalMs }} ms</n-gi>
                  </n-grid>
                  <div v-if="hreqResult.redirects.length" class="txt-label">
                    {{ t('toolbox.redirectsN', { n: hreqResult.redirects.length }) }} → {{ hreqResult.finalUrl }}
                  </div>
                  <n-space align="center" justify="space-between" style="width: 100%">
                    <div class="txt-label">{{ t('toolbox.responseHeaders') }}</div>
                    <n-button size="tiny" quaternary @click="copy(Object.entries(hreqResult.headers).map(([k, v]) => `${k}: ${v}`).join('\n'))">{{ t('toolbox.copy') }}</n-button>
                  </n-space>
                  <n-input
                    type="textarea"
                    :autosize="{ minRows: 5 }"
                    readonly
                    :value="Object.entries(hreqResult.headers).map(([k, v]) => `${k}: ${v}`).join('\n')"
                  />
                  <n-space align="center" justify="space-between" style="width: 100%">
                    <div class="txt-label">{{ t('toolbox.bodyLabel') }}</div>
                    <n-button size="tiny" quaternary @click="copy(hreqResult.bodySnippet)">{{ t('toolbox.copy') }}</n-button>
                  </n-space>
                  <n-input type="textarea" :autosize="{ minRows: 5 }" readonly :value="hreqResult.bodySnippet" />
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="tcping" :tab="t('toolbox.tcpingTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="tcHost" :placeholder="t('toolbox.host')" style="width: 220px" />
                  <n-input-number v-model:value="tcPort" :min="1" :max="65535" style="width: 120px" />
                  <n-input-number v-model:value="tcCount" :min="1" :max="100" style="width: 100px" />
                  <n-input-number v-model:value="tcInterval" :min="0" :max="5000" :step="50" style="width: 120px" />
                  <n-button size="small" type="primary" :loading="tcRunning" @click="doTCPing">{{ t('toolbox.tcpingStart') }}</n-button>
                </n-space>
                <template v-if="tcResult">
                  <n-space align="center" :wrap="true">
                    <n-tag size="small" :type="tcResult.lossPercent === 0 ? 'success' : tcResult.lossPercent < 100 ? 'warning' : 'error'">
                      {{ t('toolbox.packetLossN', { v: tcResult.lossPercent.toFixed(1) }) }}
                    </n-tag>
                    <span style="opacity: 0.7">min/avg/max = {{ tcResult.min || '-' }} / {{ tcResult.avg || '-' }} / {{ tcResult.max || '-' }}</span>
                  </n-space>
                  <n-input
                    type="textarea"
                    :autosize="{ minRows: 4 }"
                    readonly
                    :value="tcResult.samples.map((s) => `#${s.seq} ${s.open ? s.latency : t('toolbox.tcpingFail') + (s.error ? ': ' + s.error : '')}`).join('\n')"
                  />
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="whois" :tab="t('toolbox.whoisTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="whoisDomain" :placeholder="t('toolbox.domainOnly')" style="width: 280px" @keyup.enter="doWhois" />
                  <n-input-number v-model:value="whoisTimeout" :min="2000" :max="30000" :step="1000" style="width: 130px" />
                  <n-button size="small" type="primary" :loading="whoisRunning" @click="doWhois">{{ t('toolbox.query') }}</n-button>
                </n-space>
                <template v-if="whoisResult">
                  <n-grid :cols="2" :x-gap="12" :y-gap="6">
                    <n-gi><span class="txt-label">{{ t('toolbox.whoisRegistrar') }}</span>{{ whoisResult.registrar || '-' }}</n-gi>
                    <n-gi><span class="txt-label">{{ t('toolbox.whoisRegistrant') }}</span>{{ whoisResult.registrant || '-' }}</n-gi>
                    <n-gi><span class="txt-label">{{ t('toolbox.whoisCreated') }}</span>{{ whoisResult.created || '-' }}</n-gi>
                    <n-gi><span class="txt-label">{{ t('toolbox.whoisExpires') }}</span>{{ whoisResult.expires || '-' }}</n-gi>
                    <n-gi><span class="txt-label">{{ t('toolbox.whoisUpdated') }}</span>{{ whoisResult.updated || '-' }}</n-gi>
                    <n-gi>
                      <span class="txt-label">{{ t('toolbox.whoisRemaining') }}</span>
                      <n-tag v-if="whoisResult.expiryKnown" size="small" :type="whoisResult.daysToExpiry < 30 ? 'warning' : 'success'">{{ t('toolbox.daysUnit', { n: whoisResult.daysToExpiry }) }}</n-tag>
                      <span v-else>-</span>
                    </n-gi>
                  </n-grid>
                  <div v-if="whoisResult.nameServers.length">
                    <div class="txt-label">{{ t('toolbox.whoisNS') }}</div>
                    <n-space :wrap="true">
                      <n-tag v-for="ns in whoisResult.nameServers" :key="ns" size="tiny" :bordered="false">{{ ns }}</n-tag>
                    </n-space>
                  </div>
                  <div v-if="whoisResult.status.length">
                    <div class="txt-label">{{ t('toolbox.whoisStatus') }}</div>
                    <n-space :wrap="true">
                      <n-tag v-for="st in whoisResult.status" :key="st" size="tiny" :bordered="false">{{ st }}</n-tag>
                    </n-space>
                  </div>
                  <n-space align="center">
                    <n-button size="tiny" quaternary @click="whoisShowRaw = !whoisShowRaw">{{ t('toolbox.whoisToggleRaw') }}</n-button>
                    <n-button size="tiny" quaternary @click="copy(whoisResult.raw)">{{ t('toolbox.copy') }}</n-button>
                  </n-space>
                  <n-input v-if="whoisShowRaw" type="textarea" :autosize="{ minRows: 6, maxRows: 16 }" readonly :value="whoisResult.raw" />
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="iplookup" :tab="t('toolbox.ipTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <n-radio-group v-model:value="ipBatchMode" size="small">
                    <n-radio-button :value="false">{{ t('toolbox.ipSingle') }}</n-radio-button>
                    <n-radio-button :value="true">{{ t('toolbox.ipBatch') }}</n-radio-button>
                  </n-radio-group>
                  <span class="txt-label">{{ t('toolbox.ipTimeout') }}</span>
                  <n-input-number v-model:value="ipTimeout" :min="1" :max="30" :step="1" size="small" style="width: 110px">
                    <template #suffix>s</template>
                  </n-input-number>
                </n-space>

                <template v-if="!ipBatchMode">
                  <n-space align="center">
                    <n-input v-model:value="ipQuery" :placeholder="t('toolbox.ipPlaceholder')" style="width: 340px" @keyup.enter="doIPLookup" />
                    <n-button size="small" type="primary" :loading="ipRunning" @click="doIPLookup">{{ t('toolbox.ipQuery') }}</n-button>
                  </n-space>
                  <div class="tb-hint">{{ t('toolbox.ipEmptyHint') }}</div>
                  <template v-if="ipResult">
                    <n-alert v-if="!ipResult.ok" type="warning" :show-icon="false">{{ ipResult.message || t('toolbox.ipFail') }}</n-alert>
                    <template v-else>
                      <n-space align="center">
                        <n-tag size="small" type="success" :bordered="false">{{ ipResult.query }}</n-tag>
                        <n-tag size="small" :bordered="false">{{ ipLocation(ipResult) || '-' }}</n-tag>
                        <n-button size="tiny" quaternary @click="copy(`${ipResult.query} ${ipLocation(ipResult)} ${ipResult.isp}`)">{{ t('toolbox.copy') }}</n-button>
                      </n-space>
                      <n-grid :cols="3" :x-gap="12" :y-gap="6">
                        <n-gi><span class="txt-label">{{ t('toolbox.ipCountry') }}</span>{{ ipResult.country || '-' }}<span v-if="ipResult.countryCode"> ({{ ipResult.countryCode }})</span></n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipRegion') }}</span>{{ ipResult.region || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipCity') }}</span>{{ ipResult.city || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipZip') }}</span>{{ ipResult.zip || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipTimezone') }}</span>{{ ipResult.timezone || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipCoords') }}</span>{{ ipResult.lat }}, {{ ipResult.lon }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipIsp') }}</span>{{ ipResult.isp || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipOrg') }}</span>{{ ipResult.org || '-' }}</n-gi>
                        <n-gi><span class="txt-label">{{ t('toolbox.ipAs') }}</span>{{ ipResult.as || '-' }}</n-gi>
                      </n-grid>
                    </template>
                  </template>
                </template>

                <template v-else>
                  <n-input
                    v-model:value="ipBatchText"
                    type="textarea"
                    :autosize="{ minRows: 4, maxRows: 10 }"
                    :placeholder="t('toolbox.ipBatchPlaceholder')"
                  />
                  <n-space align="center">
                    <n-button size="small" type="primary" :loading="ipRunning" @click="doIPLookupBatch">{{ t('toolbox.ipQuery') }}</n-button>
                    <n-button size="small" quaternary @click="ipBatchResults = []">{{ t('toolbox.imgClear') }}</n-button>
                    <n-tag v-if="ipBatchResults.length" size="small" :bordered="false">{{ ipBatchResults.length }}</n-tag>
                  </n-space>
                  <n-data-table
                    v-if="ipBatchResults.length"
                    :columns="ipBatchColumns"
                    :data="ipBatchRows"
                    :bordered="false"
                    size="small"
                    :max-height="360"
                    :row-key="(row: any) => row.input"
                  />
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="certgen" :tab="t('toolbox.certgenTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">CN</span>
                  <n-input v-model:value="cgCN" size="small" style="width: 180px" />
                  <span class="txt-label">O</span>
                  <n-input v-model:value="cgOrg" size="small" style="width: 140px" />
                  <span class="txt-label">OU</span>
                  <n-input v-model:value="cgOU" size="small" style="width: 120px" />
                  <span class="txt-label">C</span>
                  <n-input v-model:value="cgCountry" size="small" style="width: 70px" />
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.cgDNS') }}</span>
                  <n-input v-model:value="cgDNS" size="small" style="width: 260px" />
                  <span class="txt-label">{{ t('toolbox.cgIPs') }}</span>
                  <n-input v-model:value="cgIPs" size="small" style="width: 160px" />
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.cgDays') }}</span>
                  <n-input-number v-model:value="cgDays" :min="1" :max="3650" size="small" style="width: 110px" />
                  <n-select v-model:value="cgKeyType" :options="cgKeyTypes" size="small" style="width: 150px" />
                  <n-checkbox v-model:checked="cgIsCA" size="small">{{ t('toolbox.cgIsCA') }}</n-checkbox>
                  <n-button size="small" type="primary" :loading="cgRunning" @click="doGenerateCert">{{ t('toolbox.cgGenerateCert') }}</n-button>
                  <n-button size="small" :loading="cgRunning" @click="doGenerateCSR">{{ t('toolbox.cgGenerateCSR') }}</n-button>
                </n-space>
                <template v-if="cgResult">
                  <n-grid :cols="2" :x-gap="12" :y-gap="6">
                    <n-gi><span class="txt-label">{{ t('toolbox.cgSubject') }}</span>{{ cgResult.subject }}</n-gi>
                    <n-gi><span class="txt-label">{{ t('toolbox.cgKeyType') }}</span>{{ cgResult.keyType }}</n-gi>
                    <n-gi v-if="cgResult.notBefore"><span class="txt-label">{{ t('toolbox.validity') }}</span>{{ cgResult.notBefore }} ~ {{ cgResult.notAfter }}</n-gi>
                    <n-gi><span class="txt-label">SAN</span>{{ cgResult.sans.join(', ') }}</n-gi>
                  </n-grid>
                  <n-space align="center" justify="space-between" style="width: 100%">
                    <div class="txt-label">{{ cgResult.certPem.includes('CERTIFICATE REQUEST') ? 'CSR' : 'Certificate' }}</div>
                    <n-space>
                      <n-button size="tiny" quaternary @click="copy(cgResult.certPem)">{{ t('toolbox.copy') }}</n-button>
                      <n-button size="tiny" quaternary @click="downloadText('cert.pem', cgResult.certPem)">{{ t('toolbox.download') }}</n-button>
                    </n-space>
                  </n-space>
                  <n-input type="textarea" :autosize="{ minRows: 5, maxRows: 12 }" readonly :value="cgResult.certPem" />
                  <n-space align="center" justify="space-between" style="width: 100%">
                    <div class="txt-label">{{ t('toolbox.cgPrivateKey') }}</div>
                    <n-space>
                      <n-button size="tiny" quaternary @click="copy(cgResult.keyPem)">{{ t('toolbox.copy') }}</n-button>
                      <n-button size="tiny" quaternary @click="downloadText('key.pem', cgResult.keyPem)">{{ t('toolbox.download') }}</n-button>
                    </n-space>
                  </n-space>
                  <n-input type="textarea" :autosize="{ minRows: 5, maxRows: 12 }" readonly :value="cgResult.keyPem" />
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="connstring" :tab="t('toolbox.connstringTab')">
            <n-card size="small">
              <n-space vertical>
                <div class="txt-label">{{ t('toolbox.csParseTitle') }}</div>
                <n-space align="center">
                  <n-input v-model:value="csRaw" :placeholder="t('toolbox.csRawPlaceholder')" style="width: 560px" @keyup.enter="doParseConn" />
                  <n-button size="small" type="primary" @click="doParseConn">{{ t('toolbox.csParse') }}</n-button>
                </n-space>
                <n-grid v-if="csParsed" :cols="2" :x-gap="12" :y-gap="6">
                  <n-gi><span class="txt-label">{{ t('toolbox.csKind') }}</span>{{ csParsed.kind }}</n-gi>
                  <n-gi><span class="txt-label">{{ t('toolbox.host') }}</span>{{ csParsed.host }}</n-gi>
                  <n-gi><span class="txt-label">{{ t('toolbox.portField') }}</span>{{ csParsed.port }}</n-gi>
                  <n-gi><span class="txt-label">{{ t('toolbox.csDatabase') }}</span>{{ csParsed.database || '-' }}</n-gi>
                  <n-gi><span class="txt-label">{{ t('toolbox.csUser') }}</span>{{ csParsed.username || '-' }}</n-gi>
                  <n-gi><span class="txt-label">{{ t('toolbox.csPassword') }}</span>{{ csParsed.password || '-' }}</n-gi>
                  <n-gi v-if="Object.keys(csParsed.params).length"><span class="txt-label">{{ t('toolbox.csParams') }}</span>{{ Object.entries(csParsed.params).map(([k, v]) => `${k}=${v}`).join(', ') }}</n-gi>
                </n-grid>
                <n-divider style="margin: 4px 0" />
                <div class="txt-label">{{ t('toolbox.csBuildTitle') }}</div>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <n-select v-model:value="csKind" :options="csKinds" size="small" style="width: 120px" />
                  <n-input v-model:value="csHost" size="small" :placeholder="t('toolbox.host')" style="width: 160px" />
                  <n-input-number v-model:value="csPort" :min="0" :max="65535" size="small" style="width: 110px" />
                  <n-input v-model:value="csDatabase" size="small" :placeholder="t('toolbox.csDatabase')" style="width: 130px" />
                </n-space>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <n-input v-model:value="csUser" size="small" :placeholder="t('toolbox.csUser')" style="width: 130px" />
                  <n-input v-model:value="csPassword" size="small" :placeholder="t('toolbox.csPassword')" style="width: 130px" type="password" show-password-on="click" />
                  <n-input v-model:value="csParams" size="small" :placeholder="t('toolbox.csParamsPlaceholder')" style="width: 260px" />
                  <n-button size="small" type="primary" @click="doBuildConn">{{ t('toolbox.csBuild') }}</n-button>
                  <n-button size="small" quaternary @click="copy(csBuilt)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-input v-model:value="csBuilt" readonly :placeholder="t('toolbox.result')" />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="logtime" :tab="t('toolbox.logtimeTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <n-select v-model:value="ltFormatOverride" :options="ltFormatOptions" size="small" style="width: 200px" />
                  <n-button size="small" @click="doDetectLogTime">{{ t('toolbox.ltDetect') }}</n-button>
                  <span v-if="ltDetected" class="txt-label">{{ t('toolbox.ltDetected') }}: {{ ltLabel(ltDetected) }}</span>
                  <span class="txt-label">{{ t('toolbox.ltOutput') }}</span>
                  <n-select v-model:value="ltOutput" :options="ltOutputOptions" size="small" style="width: 150px" />
                  <n-button size="small" type="primary" @click="doConvertLogTime">{{ t('toolbox.ltConvert') }}</n-button>
                </n-space>
                <n-grid :cols="2" :x-gap="12">
                  <n-gi><n-input v-model:value="ltInput" type="textarea" :autosize="{ minRows: 8 }" :placeholder="t('toolbox.ltInputPlaceholder')" /></n-gi>
                  <n-gi>
                    <n-input type="textarea" :autosize="{ minRows: 8 }" readonly :placeholder="t('toolbox.result')" :value="ltResult ? ltResult.converted : ''" />
                    <n-space align="center" justify="space-between" style="width: 100%; margin-top: 4px">
                      <span v-if="ltResult" class="txt-label">{{ t('toolbox.ltCount', { n: ltResult.count }) }} · {{ ltResult.format }}</span>
                      <n-button size="tiny" quaternary @click="copy(ltResult ? ltResult.converted : '')">{{ t('toolbox.copy') }}</n-button>
                    </n-space>
                  </n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="k8s" :tab="t('toolbox.k8sTab')">
            <n-card size="small">
              <n-space vertical>
                <div class="txt-label">{{ t('toolbox.k8sConvertTitle') }}</div>
                <n-space align="center">
                  <n-input v-model:value="k8sInput" :placeholder="t('toolbox.k8sInputPlaceholder')" style="width: 180px" />
                  <span class="txt-label">→</span>
                  <n-select v-model:value="k8sToUnit" :options="k8sConvertUnitOptions" size="small" style="width: 150px" />
                  <n-button size="small" type="primary" @click="doConvertK8s">{{ t('toolbox.calculate') }}</n-button>
                  <n-input v-model:value="k8sConverted" readonly :placeholder="t('toolbox.result')" style="width: 200px" />
                  <n-button size="small" quaternary @click="copy(k8sConverted)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-divider style="margin: 4px 0" />
                <div class="txt-label">{{ t('toolbox.k8sValidateTitle') }}</div>
                <n-input v-model:value="k8sYaml" type="textarea" :autosize="{ minRows: 6 }" :placeholder="t('toolbox.k8sYamlPlaceholder')" />
                <n-button size="small" type="primary" @click="doValidateK8s">{{ t('toolbox.k8sValidate') }}</n-button>
                <template v-if="k8sValidate">
                  <n-tag size="small" :type="k8sValidate.ok ? 'success' : 'warning'">
                    {{ k8sValidate.ok ? t('toolbox.k8sOK') : t('toolbox.k8sIssuesN', { n: k8sValidate.issues.length }) }}
                  </n-tag>
                  <n-space v-if="k8sValidate.entries.length" vertical size="small">
                    <div v-for="(e, i) in k8sValidate.entries" :key="i" class="k8s-entry">
                      <n-tag size="tiny" :type="e.scope === 'requests' ? 'info' : 'warning'">{{ e.scope }}</n-tag>
                      <span class="txt-label">{{ e.resource }}</span>
                      <span>{{ e.raw }}</span>
                      <span class="txt-label">→</span>
                      <span v-if="e.resource === 'cpu'">{{ e.cpu }} ({{ e.cpuPlain }})</span>
                      <span v-else>{{ e.memory }} ({{ e.memoryBytes }} bytes)</span>
                      <span v-if="e.note" class="k8s-note">⚠ {{ detailText(e.note) }}</span>
                    </div>
                  </n-space>
                  <div v-for="(iss, i) in k8sValidate.issues" :key="i" class="ssl-issue">⚠ {{ k8sIssueText(iss) }}</div>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="caps" :tab="t('toolbox.capsTab')">
            <n-space vertical>
              <n-card size="small" :title="t('toolbox.capsSizeTitle')">
                <n-space align="center" :wrap="true">
                  <n-input-number v-model:value="capsSizeValue" size="small" style="width: 130px" :placeholder="t('toolbox.capsValue')" />
                  <n-select v-model:value="capsSizeFrom" :options="sizeUnitOptions" size="small" style="width: 100px" />
                  <span class="txt-label">→</span>
                  <n-select v-model:value="capsSizeTo" :options="sizeUnitOptions" size="small" style="width: 100px" />
                  <n-button size="small" type="primary" @click="doConvertSize">{{ t('toolbox.capsConvert') }}</n-button>
                </n-space>
                <template v-if="capsSizeResult">
                  <n-divider style="margin: 8px 0" />
                  <n-descriptions :column="2" size="small" label-placement="left" bordered>
                    <n-descriptions-item :label="t('toolbox.capsConverted')">{{ capsSizeResult.text }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsBase')">{{ capsSizeResult.base }} B</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsDecimal')">{{ capsSizeResult.decimal }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsBinary')">{{ capsSizeResult.binary }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>

              <n-card size="small" :title="t('toolbox.capsRateTitle')">
                <n-space align="center" :wrap="true">
                  <n-input-number v-model:value="capsRateValue" size="small" style="width: 130px" :placeholder="t('toolbox.capsValue')" />
                  <n-select v-model:value="capsRateFrom" :options="rateUnitOptions" size="small" style="width: 100px" />
                  <span class="txt-label">→</span>
                  <n-select v-model:value="capsRateTo" :options="rateUnitOptions" size="small" style="width: 100px" />
                  <n-button size="small" type="primary" @click="doConvertRate">{{ t('toolbox.capsConvert') }}</n-button>
                </n-space>
                <template v-if="capsRateResult">
                  <n-divider style="margin: 8px 0" />
                  <n-descriptions :column="2" size="small" label-placement="left" bordered>
                    <n-descriptions-item :label="t('toolbox.capsConverted')">{{ capsRateResult.text }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsBase')">{{ capsRateResult.base }} bps</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsDecimal')">{{ capsRateResult.decimal }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsBinary')">{{ capsRateResult.binary }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>

              <n-card size="small" :title="t('toolbox.capsTransferTitle')">
                <n-space align="center" :wrap="true">
                  <span class="txt-label">{{ t('toolbox.capsSize') }}</span>
                  <n-input-number v-model:value="capsTransferSize" size="small" style="width: 110px" />
                  <n-select v-model:value="capsTransferSizeUnit" :options="sizeUnitOptions" size="small" style="width: 90px" />
                  <span class="txt-label">{{ t('toolbox.capsSpeed') }}</span>
                  <n-input-number v-model:value="capsTransferSpeed" size="small" style="width: 110px" />
                  <n-select v-model:value="capsTransferSpeedUnit" :options="rateUnitOptions" size="small" style="width: 100px" />
                  <span class="txt-label">{{ t('toolbox.capsEfficiency') }}</span>
                  <n-input-number v-model:value="capsTransferEff" size="small" style="width: 90px" :min="1" :max="100" />
                  <n-button size="small" type="primary" @click="doEstimateTransfer">{{ t('toolbox.capsCalc') }}</n-button>
                </n-space>
                <template v-if="capsTransferResult">
                  <n-divider style="margin: 8px 0" />
                  <n-descriptions :column="2" size="small" label-placement="left" bordered>
                    <n-descriptions-item :label="t('toolbox.capsDuration')">{{ capsTransferResult.duration }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsSustained')">{{ capsTransferResult.sustainedBy }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsSize')">{{ capsTransferResult.payload }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsSpeed')">{{ capsTransferResult.linkSpeed }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>

              <n-card size="small" :title="t('toolbox.capsRaidTitle')">
                <n-space align="center" :wrap="true">
                  <span class="txt-label">{{ t('toolbox.capsDisks') }}</span>
                  <n-input-number v-model:value="capsRaidDisks" size="small" style="width: 90px" :min="1" />
                  <span class="txt-label">{{ t('toolbox.capsDiskSize') }}</span>
                  <n-input-number v-model:value="capsRaidDiskSize" size="small" style="width: 110px" :min="0.1" />
                  <n-select v-model:value="capsRaidLevel" :options="raidLevelOptions" size="small" style="width: 110px" />
                  <n-button size="small" type="primary" @click="doCalcRaid">{{ t('toolbox.capsCalc') }}</n-button>
                </n-space>
                <template v-if="capsRaidResult">
                  <n-divider style="margin: 8px 0" />
                  <n-descriptions :column="2" size="small" label-placement="left" bordered>
                    <n-descriptions-item :label="t('toolbox.capsRaw')">{{ capsRaidResult.rawTB }} TB</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsUsable')">{{ capsRaidResult.usableTB }} TB</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsLost')">{{ capsRaidResult.lostTB }} TB</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsEfficiencyOut')">{{ (capsRaidResult.efficiency * 100).toFixed(1) }}%</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsTolerance')">{{ capsRaidResult.tolerance }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsRaidLevel')">{{ capsRaidResult.level }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>

              <n-card size="small" :title="t('toolbox.capsTrafficTitle')">
                <n-space align="center" :wrap="true">
                  <span class="txt-label">{{ t('toolbox.capsBandwidth') }}</span>
                  <n-input-number v-model:value="capsTrafficBw" size="small" style="width: 110px" :min="0.1" />
                  <span class="txt-label">{{ t('toolbox.capsUtilization') }}</span>
                  <n-input-number v-model:value="capsTrafficUtil" size="small" style="width: 90px" :min="1" :max="100" />
                  <span class="txt-label">{{ t('toolbox.capsDays') }}</span>
                  <n-input-number v-model:value="capsTrafficDays" size="small" style="width: 90px" :min="1" />
                  <n-button size="small" type="primary" @click="doEstimateTraffic">{{ t('toolbox.capsCalc') }}</n-button>
                </n-space>
                <template v-if="capsTrafficResult">
                  <n-divider style="margin: 8px 0" />
                  <n-descriptions :column="2" size="small" label-placement="left" bordered>
                    <n-descriptions-item :label="t('toolbox.capsEffective')">{{ capsTrafficResult.effectiveMbps }} Mbps</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsSustained')">{{ capsTrafficResult.bytesPerSec.toFixed(2) }} B/s</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsPerDay')">{{ capsTrafficResult.perDayText }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsMonthly')">{{ capsTrafficResult.monthlyText }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsTotal')">{{ capsTrafficResult.totalText }}</n-descriptions-item>
                    <n-descriptions-item :label="t('toolbox.capsDays')">{{ capsTrafficResult.days }}</n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>
            </n-space>
          </n-tab-pane>
    <n-tab-pane name="dns" :tab="t('toolbox.dnsTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="dnsName" :placeholder="t('toolbox.domainOnly')" style="width: 260px" />
                  <n-select v-model:value="dnsRecord" :options="dnsRecords" style="width: 120px" />
                  <n-button size="small" type="primary" @click="doDNS">{{ t('toolbox.query') }}</n-button>
                </n-space>
                <n-input
                  type="textarea"
                  :autosize="{ minRows: 4 }"
                  readonly
                  :value="dnsResults.join('\n')"
                  :placeholder="t('toolbox.result')"
                />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="subnet" :tab="t('toolbox.subnetTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="cidr" placeholder="192.168.1.0/24" style="width: 220px" />
                  <n-button size="small" type="primary" @click="doSubnet">{{ t('toolbox.calculate') }}</n-button>
                </n-space>
                <n-grid v-if="subnet" :cols="2" :x-gap="12" :y-gap="8">
                  <n-gi>{{ t('toolbox.subnet', { v: subnet.network }) }}</n-gi>
                  <n-gi>{{ t('toolbox.mask', { v: subnet.mask }) }}</n-gi>
                  <n-gi>{{ t('toolbox.broadcast', { v: subnet.broadcast }) }}</n-gi>
                  <n-gi>{{ t('toolbox.prefix', { v: subnet.prefix }) }}</n-gi>
                  <n-gi>{{ t('toolbox.hostStart', { v: subnet.firstHost }) }}</n-gi>
                  <n-gi>{{ t('toolbox.hostEnd', { v: subnet.lastHost }) }}</n-gi>
                  <n-gi>{{ t('toolbox.hostCount', { v: subnet.hostCount }) }}</n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="jwt" :tab="t('toolbox.jwtTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input v-model:value="jwtInput" type="textarea" :autosize="{ minRows: 3 }" :placeholder="t('toolbox.jwtPlaceholder')" />
                </n-space>
                <n-space align="center">
                  <n-button size="small" type="primary" @click="decodeJWT">{{ t('toolbox.decode') }}</n-button>
                </n-space>
                <template v-if="jwtResult">
                  <n-space align="center">
                    <n-tag :type="jwtResult.expired ? 'error' : 'success'" size="small">
                      {{ jwtResult.expired ? t('toolbox.jwtExpired') : t('toolbox.jwtActive') }}
                    </n-tag>
                    <n-tag size="small">{{ jwtResult.alg }}</n-tag>
                    <n-tag v-if="jwtResult.expiresIn" size="small">{{ t('toolbox.jwtExpiresIn') }}: {{ jwtResult.expiresIn }}</n-tag>
                  </n-space>
                  <n-grid :cols="2" :x-gap="12">
                    <n-gi>
                      <div class="tb-label">{{ t('toolbox.jwtHeader') }}</div>
                      <n-input :value="jwtResult.headerJson" type="textarea" :autosize="{ minRows: 5 }" readonly />
                    </n-gi>
                    <n-gi>
                      <div class="tb-label">{{ t('toolbox.jwtPayload') }}</div>
                      <n-input :value="jwtResult.payloadJson" type="textarea" :autosize="{ minRows: 5 }" readonly />
                    </n-gi>
                  </n-grid>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="cron" :tab="t('toolbox.cronTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center" :wrap="true">
                  <n-input v-model:value="cronExpr" placeholder="0 9 * * mon-fri" style="width: 320px" />
                  <n-button size="small" type="primary" @click="parseCron">{{ t('toolbox.cronParse') }}</n-button>
                </n-space>
                <n-space v-if="cronPresets.length" align="center" :wrap="true" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.cronPresets') }}</span>
                  <n-button v-for="p in cronPresets" :key="p.value" size="tiny" quaternary @click="cronExpr = p.value">
                    {{ p.label }}
                  </n-button>
                </n-space>
                <template v-if="cronResult">
                  <n-alert v-if="cronResult.description" type="info" :bordered="false">
                    {{ cronDescText(cronResult.description) }}
                  </n-alert>
                  <n-grid :cols="5" :x-gap="12">
                    <n-gi><div class="tb-label">{{ t('toolbox.cronMinute') }}</div><div>{{ cronResult.minute }}</div></n-gi>
                    <n-gi><div class="tb-label">{{ t('toolbox.cronHour') }}</div><div>{{ cronResult.hour }}</div></n-gi>
                    <n-gi><div class="tb-label">{{ t('toolbox.cronDom') }}</div><div>{{ cronResult.dayOfMonth }}</div></n-gi>
                    <n-gi><div class="tb-label">{{ t('toolbox.cronMonth') }}</div><div>{{ cronResult.month }}</div></n-gi>
                    <n-gi><div class="tb-label">{{ t('toolbox.cronDow') }}</div><div>{{ cronResult.dayOfWeek }}</div></n-gi>
                  </n-grid>
                  <div class="tb-label">{{ t('toolbox.cronNext') }}</div>
                  <div v-for="(r, i) in cronResult.nextRuns" :key="i" class="tb-mono">{{ r }}</div>
                </template>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="identity" :tab="t('toolbox.identityTab')">
            <n-card size="small">
              <n-space vertical size="large">
                <n-space align="center" item-style="display:flex;align-items:center;gap:6px">
                  <span class="txt-label">{{ t('toolbox.genCount') }}</span>
                  <n-input-number v-model:value="idCount" :min="1" :max="100" size="small" style="width: 110px" />
                  <n-button size="small" type="primary" @click="genEmails">{{ t('toolbox.randomEmail') }}</n-button>
                  <n-button size="small" type="primary" @click="genPhones">{{ t('toolbox.randomPhone') }}</n-button>
                  <n-button size="small" type="primary" @click="genIDCards">{{ t('toolbox.randomIdCard') }}</n-button>
                </n-space>
                <n-grid v-if="idEmails.length" :cols="1">
                  <n-gi>
                    <div class="txt-label">{{ t('toolbox.emailsN', { n: idEmails.length }) }}</div>
                    <n-input type="textarea" :rows="4" readonly :value="idEmails.join('\n')" />
                    <n-button size="tiny" quaternary style="margin-top: 4px" @click="copy(idEmails.join('\n'))">{{ t('toolbox.copy') }}</n-button>
                  </n-gi>
                </n-grid>
                <n-grid v-if="idPhones.length" :cols="1">
                  <n-gi>
                    <div class="txt-label">{{ t('toolbox.phonesN', { n: idPhones.length }) }}</div>
                    <n-input type="textarea" :rows="4" readonly :value="idPhones.join('\n')" />
                    <n-button size="tiny" quaternary style="margin-top: 4px" @click="copy(idPhones.join('\n'))">{{ t('toolbox.copy') }}</n-button>
                  </n-gi>
                </n-grid>
                <n-grid v-if="idCards.length" :cols="1">
                  <n-gi>
                    <div class="txt-label">{{ t('toolbox.idcardsN', { n: idCards.length }) }}</div>
                    <n-input type="textarea" :rows="4" readonly :value="idCards.join('\n')" />
                    <n-button size="tiny" quaternary style="margin-top: 4px" @click="copy(idCards.join('\n'))">{{ t('toolbox.copy') }}</n-button>
                  </n-gi>
                </n-grid>
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="random" :tab="t('toolbox.randomTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-input-number v-model:value="rndLen" :min="1" :max="256" style="width: 140px" />
                  <n-input v-model:value="rndCharset" :placeholder="t('toolbox.charsetPlaceholder')" style="width: 320px" />
                  <n-button size="small" type="primary" @click="doRandom">{{ t('toolbox.generate') }}</n-button>
                  <n-button size="small" quaternary @click="copy(rndOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-input v-model:value="rndOutput" readonly :placeholder="t('toolbox.result')" />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="ids" :tab="t('toolbox.idTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-select v-model:value="idKind" :options="[{ label: 'UUID v4', value: 'uuid' }, { label: 'ULID', value: 'ulid' }]" style="width: 140px" />
                  <n-input-number v-model:value="newIdCount" :min="1" :max="1000" style="width: 120px" />
                  <n-button size="small" type="primary" @click="genIDs">{{ t('toolbox.generate') }}</n-button>
                  <n-button size="small" quaternary @click="copy(idOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-input v-model:value="idOutput" type="textarea" :autosize="{ minRows: 8 }" :placeholder="t('toolbox.result')" readonly />
              </n-space>
            </n-card>
          </n-tab-pane>
    <n-tab-pane name="url" :tab="t('toolbox.urlTab')">
            <n-card size="small">
              <n-space vertical>
                <n-space align="center">
                  <n-select
                    v-model:value="urlMode"
                    :options="[{ label: t('toolbox.urlQuery') + ' / Component', value: 'query' }, { label: t('toolbox.urlPath'), value: 'path' }]"
                    style="width: 200px"
                  />
                  <n-button size="small" type="primary" @click="urlEncode">{{ t('toolbox.encode') }}</n-button>
                  <n-button size="small" @click="urlDecode">{{ t('toolbox.decode') }}</n-button>
                  <n-button size="small" quaternary @click="copy(urlOutput)">{{ t('toolbox.copy') }}</n-button>
                </n-space>
                <n-input v-model:value="urlInput" type="textarea" :autosize="{ minRows: 4 }" :placeholder="t('toolbox.urlPlaceholder')" />
                <n-input v-model:value="urlOutput" type="textarea" :autosize="{ minRows: 4 }" :placeholder="t('toolbox.result')" readonly />
              </n-space>
            </n-card>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
    </n-tabs>

    <n-modal
      v-model:show="pwSaveModal"
      preset="card"
      :title="t('toolbox.pwSaveModalTitle')"
      style="width: 420px"
    >
      <n-space vertical>
        <div class="txt-label">{{ t('toolbox.pwField') }}</div>
        <n-input :value="pwOutput" readonly />
        <div class="txt-label">{{ t('toolbox.pwTitleField') }}</div>
        <n-input v-model:value="pwSaveTitle" :placeholder="t('toolbox.pwTitlePlaceholder')" />
        <div class="txt-label">{{ t('toolbox.pwNoteField') }}</div>
        <n-input v-model:value="pwSaveNote" type="textarea" :rows="3" :placeholder="t('toolbox.pwNoteOptional')" />
        <n-space justify="end">
          <n-button size="small" @click="pwSaveModal = false">{{ t('toolbox.pwCancelBtn') }}</n-button>
          <n-button size="small" type="primary" :loading="pwSaving" @click="doSavePassword">{{ t('toolbox.pwSaveBtn') }}</n-button>
        </n-space>
      </n-space>
    </n-modal>
  </div>
</template>

<style scoped>
.tb-subtabs :deep(.n-tabs-rail) {
  flex-wrap: wrap;
  row-gap: 4px;
}
.tb-subtabs :deep(.n-tabs-tab-wrapper) {
  flex-basis: auto;
  flex-grow: 0;
}
.tb-subtabs :deep(.n-tabs-tab) {
  padding: 5px 14px;
  white-space: nowrap;
}
.tb-subtabs :deep(.n-tabs-capsule) {
  display: none;
}
.tb-subtabs :deep(.n-tabs-tab.n-tabs-tab--active) {
  background-color: var(--n-tab-color-segment);
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.08);
}
.tb-subtabs :deep(.n-tabs-pane-wrapper) {
  width: 100%;
  flex-basis: 100%;
}
.tb-hint {
  font-size: 12px;
  opacity: 0.5;
}
.wm-filelist {
  max-height: 200px;
  overflow: auto;
  margin-top: 6px;
}
.wm-preview {
  margin-top: 8px;
  max-height: 460px;
  overflow: auto;
  text-align: center;
}
.wm-preview img {
  max-width: 100%;
  border: 1px solid var(--n-border-color, #ddd);
  border-radius: 6px;
}
.imgcmp-table {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 320px;
  overflow: auto;
}
.imgcmp-row {
  display: grid;
  grid-template-columns: 1fr 90px 150px 96px;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 6px;
  font-size: 12px;
}
.imgcmp-row > :last-child {
  justify-self: end;
}
.imgcmp-row:not(.imgcmp-head):nth-child(odd) {
  background: rgba(255, 255, 255, 0.04);
}
.imgcmp-head {
  font-weight: 600;
  opacity: 0.6;
}
.imgcmp-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.txt-label {
  font-size: 12px;
  opacity: 0.65;
}
.pw-dim {
  font-size: 11px;
  opacity: 0.45;
}
.pw-output {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 15px;
  letter-spacing: 0.5px;
}
.ssl-summary {
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 16px 20px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
}
.ssl-grade {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 84px;
}
.ssl-grade-letter {
  font-size: 44px;
  font-weight: 700;
  line-height: 1;
}
.ssl-grade-score {
  font-size: 12px;
  opacity: 0.7;
  margin-top: 6px;
}
.ssl-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  min-width: 0;
}
.ssl-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.ssl-label {
  display: inline-block;
  min-width: 76px;
  opacity: 0.55;
  font-size: 12px;
}
.ssl-dim {
  opacity: 0.55;
  font-size: 12px;
}
.ssl-issues {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(208, 48, 80, 0.08);
  border: 1px solid rgba(208, 48, 80, 0.25);
  font-size: 13px;
}
.ssl-ok {
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(24, 160, 88, 0.08);
  border: 1px solid rgba(24, 160, 88, 0.25);
  font-size: 13px;
}
.ssl-section-title {
  font-size: 13px;
  font-weight: 600;
  margin: 8px 0 8px;
  opacity: 0.85;
}
.ssl-protocol-table {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.ssl-protocol-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
  font-size: 13px;
}
.ssl-proto-name {
  min-width: 70px;
  font-weight: 500;
}
.ssl-cipher {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ssl-chain {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.ssl-cert {
  padding: 12px 16px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
}
.ssl-cert-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.ssl-cert-badge {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 10px;
  background: rgba(99, 226, 183, 0.15);
  color: #63e2b7;
}
.ssl-cert-name {
  font-weight: 600;
  font-size: 13px;
}
.ssl-cert-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 6px 16px;
  font-size: 12px;
}
.ssl-san-list {

  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.k8s-entry {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 13px;
}

.k8s-note {
  color: #f0a020;
  font-size: 12px;
}

.tb-label {
  font-size: 12px;
  opacity: 0.65;
  margin-bottom: 2px;
}

.tb-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.tb-diff {
  padding: 0 4px;
  border-radius: 3px;
}

.tb-diff-add {
  background: rgba(24, 160, 88, 0.15);
}

.tb-diff-del {
  background: rgba(208, 48, 80, 0.15);
}

.tb-diff-sign {
  display: inline-block;
  width: 14px;
  opacity: 0.6;
}
</style>
