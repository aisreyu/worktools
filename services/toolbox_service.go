package services

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/toolbox"
	"github.com/aisreyu/worktools/internal/vault"
)

// ToolboxService exposes stateless ops utilities to the UI.
type ToolboxService struct {
	vault *vault.Vault
}

// NewToolboxService creates the toolbox service.
func NewToolboxService(v *vault.Vault) *ToolboxService { return &ToolboxService{vault: v} }

// ---- encoding ----

// Base64Encode encodes a string.
func (s *ToolboxService) Base64Encode(in string) string { return toolbox.Base64Encode(in) }

// Base64Decode decodes a string.
func (s *ToolboxService) Base64Decode(in string) (string, error) { return toolbox.Base64Decode(in) }

// HexEncode encodes a string as hex.
func (s *ToolboxService) HexEncode(in string) string { return toolbox.HexEncode(in) }

// HexDecode decodes hex into a string.
func (s *ToolboxService) HexDecode(in string) (string, error) { return toolbox.HexDecode(in) }

// Hash returns the hex digest of a string.
func (s *ToolboxService) Hash(algo, in string) (string, error) { return toolbox.Hash(algo, in) }

// ---- json ----

// JSONFormat formats or compacts JSON.
func (s *ToolboxService) JSONFormat(in string, indent bool) (string, error) {
	return toolbox.JSONFormat(in, indent)
}

// ---- time ----

// TimeFromUnix converts a unix timestamp to RFC3339.
func (s *ToolboxService) TimeFromUnix(ts string, millis bool) (string, error) {
	return toolbox.TimeFromUnix(ts, millis)
}

// UnixFromTime converts an RFC3339 string to a unix timestamp.
func (s *ToolboxService) UnixFromTime(in string) (int64, error) { return toolbox.UnixFromTime(in) }

// ---- net ----

// PortScan probes a list of TCP ports.
func (s *ToolboxService) PortScan(host string, ports []int, timeoutMS int) []toolbox.PortResult {
	return toolbox.PortScan(context.Background(), host, ports, timeoutMS)
}

// HTTPProbe performs an HTTP GET probe.
func (s *ToolboxService) HTTPProbe(url string, timeoutMS int) (toolbox.HTTPResult, error) {
	return toolbox.HTTPProbe(context.Background(), url, timeoutMS)
}

// SSLCheck inspects a server's TLS certificate chain and protocol support.
func (s *ToolboxService) SSLCheck(host string, port int, timeoutMS int) (toolbox.SSLCheckResult, error) {
	return toolbox.SSLCheck(context.Background(), host, port, timeoutMS)
}

// DNSLookup resolves a DNS record.
func (s *ToolboxService) DNSLookup(name, record string) ([]string, error) {
	return toolbox.DNSLookup(context.Background(), name, record)
}

// ParseSubnet computes CIDR details.
func (s *ToolboxService) ParseSubnet(cidr string) (toolbox.SubnetInfo, error) {
	return toolbox.ParseSubnet(cidr)
}

// ---- toolbox ops enhancements ----

// ExpandPortSpec expands a port spec like "80,443,8000-8010" into a sorted list.
func (s *ToolboxService) ExpandPortSpec(spec string) ([]int, error) {
	return toolbox.ExpandPortSpec(spec)
}

// PortPresets returns the built-in one-click port scan groups.
func (s *ToolboxService) PortPresets() []toolbox.PortPreset {
	return toolbox.PortPresets()
}

// HTTPRequest performs a configurable HTTP request (lightweight curl).
func (s *ToolboxService) HTTPRequest(opts toolbox.HTTPRequestOptions) (toolbox.HTTPProbeResult, error) {
	return toolbox.HTTPRequest(context.Background(), opts)
}

// TCPing probes host:port multiple times to measure latency and loss.
func (s *ToolboxService) TCPing(host string, port, count, timeoutMS, intervalMS int) (toolbox.TCPingResult, error) {
	return toolbox.TCPing(context.Background(), host, port, count, timeoutMS, intervalMS)
}

// Whois queries registration data for a domain.
func (s *ToolboxService) Whois(domain string, timeoutMS int) (toolbox.WhoisResult, error) {
	return toolbox.Whois(context.Background(), domain, timeoutMS)
}

// GenerateSelfSignedCert creates a self-signed certificate and key in PEM form.
func (s *ToolboxService) GenerateSelfSignedCert(opts toolbox.CertGenOptions) (toolbox.CertGenResult, error) {
	return toolbox.GenerateSelfSignedCert(opts)
}

// GenerateCSR creates a certificate signing request in PEM form.
func (s *ToolboxService) GenerateCSR(opts toolbox.CertGenOptions) (toolbox.CertGenResult, error) {
	return toolbox.GenerateCSR(opts)
}

// CertPEMToInfo parses a PEM certificate into structured info.
func (s *ToolboxService) CertPEMToInfo(certPEM string) (toolbox.CertInfo, error) {
	return toolbox.CertPEMToInfo(certPEM)
}

// ParseConnString parses a database/messaging connection URL into fields.
func (s *ToolboxService) ParseConnString(raw string) (toolbox.ConnString, error) {
	return toolbox.ParseConnString(raw)
}

// BuildConnString builds a connection URL from structured fields.
func (s *ToolboxService) BuildConnString(in toolbox.ConnString) (string, error) {
	return toolbox.BuildConnString(in)
}

// DetectLogTime detects the timestamp format used in log text.
func (s *ToolboxService) DetectLogTime(text string) toolbox.LogTimeFormat {
	return toolbox.DetectLogTime(text)
}

// ConvertLogTime detects/converts log timestamps to the requested output form.
func (s *ToolboxService) ConvertLogTime(text, formatID, output string) (toolbox.LogTimeResult, error) {
	return toolbox.ConvertLogTime(text, formatID, output)
}

// ConvertK8sResource converts a Kubernetes cpu/memory quantity.
func (s *ToolboxService) ConvertK8sResource(input, toUnit string) (string, error) {
	return toolbox.ConvertK8sResource(input, toUnit)
}

// ValidateResourcesYAML extracts and validates cpu/memory requests and limits.
func (s *ToolboxService) ValidateResourcesYAML(yaml string) toolbox.K8sValidationResult {
	return toolbox.ValidateResourcesYAML(yaml)
}

// RandomString generates a random string.
func (s *ToolboxService) RandomString(n int, charset string) (string, error) {
	return toolbox.RandomString(n, charset)
}

// ---- text / data transforms ----

// RowsToColumns transposes row-oriented text into column-oriented text.
func (s *ToolboxService) RowsToColumns(input, sep string) string {
	return toolbox.RowsToColumns(input, sep)
}

// ColumnsToRows transposes column-oriented text back into rows.
func (s *ToolboxService) ColumnsToRows(input, sep string) string {
	return toolbox.ColumnsToRows(input, sep)
}

// JoinLines joins non-empty lines with a separator, optionally quoting items.
func (s *ToolboxService) JoinLines(input, sep, quote string) string {
	return toolbox.JoinLines(input, sep, quote)
}

// QuoteLines quotes every line and joins them — handy for SQL IN lists.
func (s *ToolboxService) QuoteLines(input, sep, quote string) string {
	return toolbox.QuoteLines(input, sep, quote)
}

// DedupeLines removes duplicate lines, optionally sorting the result.
func (s *ToolboxService) DedupeLines(input string, sortOut bool) string {
	return toolbox.DedupeLines(input, sortOut)
}

// TextToJSON converts header+rows text into a JSON array of objects.
func (s *ToolboxService) TextToJSON(input, sep string, noHeader bool) (string, error) {
	return toolbox.TextToJSON(input, sep, noHeader)
}

// JSONToText flattens a JSON array of objects into header+rows text.
func (s *ToolboxService) JSONToText(input, sep string) (string, error) {
	return toolbox.JSONToText(input, sep)
}

// ---- password & random data ----

// GeneratePassword creates a random password with the requested complexity.
func (s *ToolboxService) GeneratePassword(opts toolbox.PasswordOptions) (string, error) {
	return toolbox.GeneratePassword(opts)
}

// JoinWithAffixes wraps every line with prefix/suffix and joins the results.
func (s *ToolboxService) JoinWithAffixes(input, prefix, suffix, sep string) string {
	return toolbox.JoinWithAffixes(input, prefix, suffix, sep)
}

// RandomEmails generates n random email addresses.
func (s *ToolboxService) RandomEmails(n int) ([]string, error) {
	return toolbox.RandomEmails(n)
}

// RandomPhones generates n random Chinese mobile numbers.
func (s *ToolboxService) RandomPhones(n int) ([]string, error) {
	return toolbox.RandomPhones(n)
}

// RandomIDCards generates n synthetic Chinese ID numbers with valid
// checksums (for testing only).
func (s *ToolboxService) RandomIDCards(n int) ([]string, error) {
	return toolbox.RandomIDCards(n)
}

// SavePasswordToVault stores a generated password into the vault as a
// secret entry, so generated credentials don't have to be re-typed.
func (s *ToolboxService) SavePasswordToVault(password, title, note string) (int64, error) {
	if s.vault.Locked() {
		return 0, vault.ErrLocked
	}
	if strings.TrimSpace(title) == "" {
		title = "生成的密码 " + time.Now().Format("2006-01-02 15:04")
	}
	e := store.SecretEntry{
		Title:    title,
		Category: "generated",
		Password: password,
		Note:     note,
	}
	return s.vault.SaveSecret(e)
}

// ---- daily tools: URL ----

// URLEncodeComponent percent-encodes a string. mode "path" keeps "/" literal,
// any other value encodes as a query component.
func (s *ToolboxService) URLEncodeComponent(in, mode string) string {
	return toolbox.URLEncodeComponent(in, mode)
}

// URLDecodeComponent percent-decodes a string.
func (s *ToolboxService) URLDecodeComponent(in, mode string) (string, error) {
	return toolbox.URLDecodeComponent(in, mode)
}

// ---- daily tools: JWT ----

// DecodeJWT decodes a JWT's header and payload without verifying the signature.
func (s *ToolboxService) DecodeJWT(token string) (toolbox.JWTResult, error) {
	return toolbox.DecodeJWT(token)
}

// ---- daily tools: cron ----

// ParseCron parses a cron expression and previews the next runs.
func (s *ToolboxService) ParseCron(expr string, count int) (toolbox.CronSchedule, error) {
	return toolbox.ParseCron(expr, count)
}

// ---- daily tools: YAML ----

// FormatYAML re-indents a YAML document.
func (s *ToolboxService) FormatYAML(in string) (string, error) { return toolbox.FormatYAML(in) }

// ValidateYAML reports whether a YAML document is well-formed.
func (s *ToolboxService) ValidateYAML(in string) error { return toolbox.ValidateYAML(in) }

// YAMLToJSON converts YAML to JSON.
func (s *ToolboxService) YAMLToJSON(in string) (string, error) { return toolbox.YAMLToJSON(in) }

// JSONToYAML converts JSON to YAML.
func (s *ToolboxService) JSONToYAML(in string) (string, error) { return toolbox.JSONToYAML(in) }

// ---- daily tools: time diff ----

// TimeDiff computes the interval between two timestamps.
func (s *ToolboxService) TimeDiff(from, to string) (toolbox.TimeDiffResult, error) {
	return toolbox.TimeDiff(from, to)
}

// Now returns the current local time as RFC3339.
func (s *ToolboxService) Now() string { return toolbox.Now() }

// ---- daily tools: identifiers ----

// UUIDs generates n random UUID v4 strings.
func (s *ToolboxService) UUIDs(n int) ([]string, error) { return toolbox.UUIDs(n) }

// ULIDs generates n random ULIDs.
func (s *ToolboxService) ULIDs(n int) ([]string, error) { return toolbox.ULIDs(n) }

// ValidateULID reports whether in is a well-formed ULID.
func (s *ToolboxService) ValidateULID(in string) bool { return toolbox.ValidateULID(in) }

// ---- daily tools: regex ----

// RegexTest runs a pattern and returns all matches with capture groups.
func (s *ToolboxService) RegexTest(pattern, input string, ignoreCase bool) toolbox.RegexResult {
	return toolbox.RegexTest(pattern, input, ignoreCase)
}

// RegexReplace replaces all matches of a pattern.
func (s *ToolboxService) RegexReplace(pattern, input, repl string, ignoreCase bool) (string, error) {
	return toolbox.RegexReplace(pattern, input, repl, ignoreCase)
}

// ---- daily tools: text diff ----

// TextDiff computes a line-level diff between two texts.
func (s *ToolboxService) TextDiff(a, b string) toolbox.DiffResult { return toolbox.TextDiff(a, b) }

// UnifiedDiff renders a unified-style diff between two texts.
func (s *ToolboxService) UnifiedDiff(a, b string) string { return toolbox.UnifiedDiff(a, b) }

// ---- daily tools: csv to sql ----

// CSVToSQL converts CSV text into INSERT statements.
func (s *ToolboxService) CSVToSQL(csv, table, dialect string, createTable bool) (string, error) {
	return toolbox.CSVToSQL(csv, table, dialect, createTable)
}

// ---- daily tools: qr ----

// QRGenerate renders text as a base64 PNG QR code.
func (s *ToolboxService) QRGenerate(text, ecLevel string, scale int) (string, error) {
	return toolbox.QRGenerate(text, ecLevel, scale)
}

// ---- ops tools: capacity & bandwidth calculators ----

// ConvertDataSize converts a data size between decimal and binary units.
func (s *ToolboxService) ConvertDataSize(value float64, from, to string) (toolbox.UnitConversion, error) {
	return toolbox.ConvertDataSize(value, from, to)
}

// ConvertDataRate converts a data rate between bit-based and byte-based units.
func (s *ToolboxService) ConvertDataRate(value float64, from, to string) (toolbox.UnitConversion, error) {
	return toolbox.ConvertDataRate(value, from, to)
}

// EstimateTransferTime estimates the time to move a payload over a link.
func (s *ToolboxService) EstimateTransferTime(size float64, sizeUnit string, speed float64, speedUnit string, efficiencyPct float64) (toolbox.TransferEstimate, error) {
	return toolbox.EstimateTransferTime(size, sizeUnit, speed, speedUnit, efficiencyPct)
}

// CalcRAIDCapacity computes usable capacity for common RAID levels.
func (s *ToolboxService) CalcRAIDCapacity(disks int, diskTB float64, level string) (toolbox.RAIDPlan, error) {
	return toolbox.CalcRAIDCapacity(disks, diskTB, level)
}

// EstimateTraffic estimates the data volume carried by a link over time.
func (s *ToolboxService) EstimateTraffic(bandwidthMbps, utilizationPct, days float64) (toolbox.TrafficEstimate, error) {
	return toolbox.EstimateTraffic(bandwidthMbps, utilizationPct, days)
}

// ---- watermark & rotate (image + PDF) ----

// WatermarkImage stamps a text watermark onto an image and writes the result to outPath.
func (s *ToolboxService) WatermarkImage(inPath, outPath string, opts toolbox.WatermarkTextOptions) error {
	return toolbox.WatermarkImage(inPath, outPath, opts)
}

// PreviewImage renders a scaled-down preview of the watermarked image as base64 PNG.
func (s *ToolboxService) PreviewImage(inPath string, opts toolbox.WatermarkTextOptions, maxDim int) (string, error) {
	return toolbox.PreviewImage(inPath, opts, maxDim)
}

// RotateImage rotates an image clockwise by degrees and writes it to outPath.
func (s *ToolboxService) RotateImage(inPath, outPath string, degrees int) error {
	return toolbox.RotateImage(inPath, outPath, degrees)
}

// PDFPageInfo returns the page count and first-page dimensions of a PDF.
func (s *ToolboxService) PDFPageInfo(inPath string) (toolbox.PDFInfo, error) {
	return toolbox.PDFPageInfo(inPath)
}

// WatermarkPDF stamps a text watermark onto selected PDF pages and writes outPath.
func (s *ToolboxService) WatermarkPDF(inPath, outPath, pages string, opts toolbox.WatermarkTextOptions) error {
	return toolbox.WatermarkPDF(inPath, outPath, pages, opts)
}

// RotatePDF rotates selected PDF pages by rotation degrees (multiple of 90).
func (s *ToolboxService) RotatePDF(inPath, outPath string, rotation int, pages string) error {
	return toolbox.RotatePDF(inPath, outPath, rotation, pages)
}

// PreviewPDF renders a preview thumbnail of a watermarked PDF as base64 PNG.
func (s *ToolboxService) PreviewPDF(inPath string, opts toolbox.WatermarkTextOptions, maxDim int) (string, error) {
	return toolbox.PreviewPDF(inPath, opts, maxDim)
}

// WatermarkOutputPath derives the output path for a file given an output dir and suffix.
func (s *ToolboxService) WatermarkOutputPath(inPath, outDir, suffix string) string {
	return toolbox.OutputPath(inPath, outDir, suffix)
}

// ---- image compression ----

// CompressImage compresses an image file and returns metadata plus a base64
// preview of the result. Nothing is written to disk.
func (s *ToolboxService) CompressImage(inPath string, opts toolbox.CompressOptions) (toolbox.CompressResult, error) {
	res, _, err := toolbox.CompressFile(inPath, opts)
	return res, err
}

// CompressImageBytes compresses an image supplied as base64 (no data-URI), for
// sources already read by the app.
func (s *ToolboxService) CompressImageBytes(name, dataB64 string, opts toolbox.CompressOptions) (toolbox.CompressResult, error) {
	res, _, err := toolbox.CompressBytes(name, dataB64, opts)
	return res, err
}

// CompressBatch compresses many images. When outDir is non-empty, each
// compressed image is written there; otherwise only metadata is returned.
func (s *ToolboxService) CompressBatch(inPaths []string, outDir string, opts toolbox.CompressOptions) ([]toolbox.CompressResult, error) {
	out := make([]toolbox.CompressResult, 0, len(inPaths))
	for _, p := range inPaths {
		res, data, err := toolbox.CompressFile(p, opts)
		if err != nil {
			return out, err
		}
		if strings.TrimSpace(outDir) != "" {
			dest := toolbox.CompressOutputPath(p, outDir, res.Mime)
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return out, errText("write output: " + err.Error())
			}
		}
		res.Data = ""
		out = append(out, res)
	}
	return out, nil
}

// CompressResultPath suggests the output path a compressed image would use.
func (s *ToolboxService) CompressResultPath(inPath, outDir, mimeType string) string {
	return toolbox.CompressOutputPath(inPath, outDir, mimeType)
}
