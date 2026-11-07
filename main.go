package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/config"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/store"
	"gitlab.tongdun.cn/anxiang.peng/notebook/internal/vault"
	"gitlab.tongdun.cn/anxiang.peng/notebook/services"
)

// The frontend build output is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

// Tray icon (PNG) embedded from the build assets.
//
//go:embed build/appicon.png
var trayIcon []byte

// lockEventName is emitted when the app is locked from outside the web UI
// (tray menu or global shortcut) so the frontend can react.
const lockEventName = "nb:lock"

func main() {
	dbPath, err := config.DBPath()
	if err != nil {
		log.Fatalf("resolve data path: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	v := vault.New(db)

	app := application.New(application.Options{
		Name:        "worktools",
		Description: "Ops desktop workbench",
		Services: []application.Service{
			application.NewService(services.NewAppService()),
			application.NewService(services.NewVaultService(v)),
			application.NewService(services.NewOTPService(v)),
			application.NewService(services.NewToolboxService(v)),
			application.NewService(services.NewNoteService(v)),
			application.NewService(services.NewFileService(v)),
			application.NewService(services.NewBackupService(v)),
			application.NewService(services.NewOpenURLService(v)),
			application.NewService(services.NewExportService()),
			application.NewService(services.NewScheduleService(v)),
			application.NewService(services.NewWorkbenchService(v)),
			application.NewService(services.NewClipboardService(v)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "WorkTools",
		Width:  1180,
		Height: 760,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 44,
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(18, 18, 24),
		URL:              "/",
	})

	installTray(app, v)
	installShortcuts(app, v)

	_ = win

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// lockVault locks the vault and notifies the frontend so it can return to the
// unlock screen.
func lockVault(app *application.App, v *vault.Vault) {
	v.Lock()
	app.Event.Emit(lockEventName)
}

// installTray adds a system-tray icon with quick actions (show, hide, lock,
// quit). The tray keeps the app reachable while the main window is hidden.
func installTray(app *application.App, v *vault.Vault) {
	menu := application.NewMenu()
	menu.Add("显示 WorkTools").OnClick(func(*application.Context) { app.Show() })
	menu.Add("隐藏窗口").OnClick(func(*application.Context) { app.Hide() })
	menu.AddSeparator()
	menu.Add("立即锁定").OnClick(func(*application.Context) { lockVault(app, v) })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { app.Quit() })

	tray := app.SystemTray.New()
	tray.SetTooltip("WorkTools")
	if len(trayIcon) > 0 {
		tray.SetIcon(trayIcon)
	}
	tray.SetMenu(menu)
	tray.OnClick(func() { app.Show() })
}

// installShortcuts registers application-wide accelerators: Cmd/Ctrl+L locks
// the vault, Cmd/Ctrl+Shift+W hides the window.
func installShortcuts(app *application.App, v *vault.Vault) {
	_ = app.GlobalShortcut.Register("CmdOrCtrl+L", func() { lockVault(app, v) })
	_ = app.GlobalShortcut.Register("CmdOrCtrl+Shift+W", func() { app.Hide() })
}
