// Thin wrappers over the generated Wails bindings so views don't import deep
// paths and we can centralise locked-vault error handling.
import * as Services from '../../bindings/github.com/aisreyu/worktools/services'
import * as Store from '../../bindings/github.com/aisreyu/worktools/internal/store'
import * as OTP from '../../bindings/github.com/aisreyu/worktools/internal/otp'
import * as Toolbox from '../../bindings/github.com/aisreyu/worktools/internal/toolbox'
import * as Holiday from '../../bindings/github.com/aisreyu/worktools/internal/holiday'

export const AppService = Services.AppService
export const WorkbenchService = Services.WorkbenchService
export const VaultService = Services.VaultService
export const OTPService = Services.OTPService
export const ToolboxService = Services.ToolboxService
export const NoteService = Services.NoteService
export const FileService = Services.FileService
export const BackupService = Services.BackupService
export const OpenURLService = Services.OpenURLService
export const ExportService = Services.ExportService
export const ScheduleService = Services.ScheduleService
export const ClipboardService = Services.ClipboardService
export const TrashService = Services.TrashService

export type VaultStatus = Services.Status
export type URLEntry = Store.URLEntry
export type SecretEntry = Store.SecretEntry
export type OTPEntry = Store.OTPEntry
export type NoteEntry = Store.NoteEntry
export type FileEntry = Store.FileEntry
export type FileCategory = Store.FileCategory
export type Category = Store.Category
export type CompressOptions = Toolbox.CompressOptions
export type CompressResult = Toolbox.CompressResult
export type ConvertOptions = Toolbox.ConvertOptions
export type ConvertResult = Toolbox.ConvertResult
export type WatermarkTextOptions = Toolbox.WatermarkTextOptions
export type PDFInfo = Toolbox.PDFInfo
export type RecoveryQuestion = Store.RecoveryQuestion
export type ScheduleEntry = Store.ScheduleEntry
export type OTPCode = OTP.Code
export type PortResult = Toolbox.PortResult
export type HTTPResult = Toolbox.HTTPResult
export type SubnetInfo = Toolbox.SubnetInfo
export type Day = Holiday.Day
export const Kind = Holiday.Kind
export type AppInfo = Services.AppInfo
export type AutoBackupStatus = Services.AutoBackupStatus
export type BackupFile = Services.BackupFile
export type TrashItem = Store.TrashItem
export type SearchHit = Toolbox.SearchHit
export type PasswordAuditFinding = Toolbox.PasswordAuditFinding
export type PasswordAuditReport = Toolbox.PasswordAuditReport
export type URLHealthResult = Toolbox.URLHealthResult
export type ImportedCredential = Toolbox.ImportedCredential
export type ImportedCredentials = Toolbox.ImportedCredentials
export type ClipboardItem = Store.ClipboardItem
export type PhoneResult = Toolbox.PhoneResult
export type IDCardResult = Toolbox.IDCardResult

export const VaultErrorLocked = 'vault is locked'

// isLockedErr detects the sentinel error the Go vault returns when locked.
export function isLockedErr(e: unknown): boolean {
  const msg = e instanceof Error ? e.message : String(e)
  return msg.includes(VaultErrorLocked)
}
