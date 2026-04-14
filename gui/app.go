package gui

import (
	"context"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type App struct {
	window      fyne.Window
	service     *RenamerService
	cfg         Config
	logEntries  []LogEntry
	statusLabel *widget.Label
	logList     *widget.List
}

type Config struct {
	WatchRoot          string
	EventDelay         int
	FileStableChecks   int
	FileStableInterval int
}

type LogEntry struct {
	Time    string
	Type    string
	Message string
}

func NewGuiApp(cfg Config) *App {
	return &App{
		cfg:        cfg,
		logEntries: []LogEntry{},
	}
}

func (a *App) Run() {
	fyneApp := app.New()
	a.window = fyneApp.NewWindow("Auto Rename")

	a.loadConfig()
	a.buildUI()

	a.window.Resize(fyne.NewSize(700, 600))
	a.window.ShowAndRun()
}

func (a *App) loadConfig() {
	a.cfg = Config{
		WatchRoot:          "/home/home/openlist/file",
		EventDelay:         3,
		FileStableChecks:   4,
		FileStableInterval: 1,
	}
}

func (a *App) buildUI() {
	statusCard := a.createStatusCard()
	dirCard := a.createDirectoryCard()
	ruleCard := a.createRuleCard()
	advancedCard := a.createAdvancedCard()
	logCard := a.createLogCard()

	content := container.NewVBox(
		statusCard,
		dirCard,
		ruleCard,
		advancedCard,
		logCard,
	)

	scroll := container.NewScroll(content)
	a.window.SetContent(scroll)
}

func (a *App) createStatusCard() *fyne.Container {
	a.statusLabel = widget.NewLabel("状态: ● 已停止")
	a.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	stopBtn := widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), func() {
		a.stopService()
	})
	stopBtn.Disable()

	restartBtn := widget.NewButtonWithIcon("重启", theme.ViewRefreshIcon(), func() {
		a.restartService()
	})

	startBtn := widget.NewButtonWithIcon("启动", theme.MediaPlayIcon(), func() {
		a.startService()
	})

	btnContainer := container.NewHBox(startBtn, stopBtn, restartBtn)

	return container.NewBorder(
		nil, nil, nil, btnContainer,
		a.statusLabel,
	)
}

func (a *App) createDirectoryCard() *fyne.Container {
	dirEntry := widget.NewEntry()
	dirEntry.SetText(a.cfg.WatchRoot)
	dirEntry.SetPlaceHolder("选择要监听的目录...")

	chooseBtn := widget.NewButtonWithIcon("选择目录", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
			if err == nil && dir != nil {
				dirEntry.SetText(dir.Path())
				a.cfg.WatchRoot = dir.Path()
			}
		}, a.window)
	})

	entryContainer := container.NewBorder(
		nil, nil, widget.NewLabel("监听目录:"), nil,
		dirEntry,
	)

	return container.NewVBox(
		widget.NewCard("", "", entryContainer),
		container.NewHBox(layout.NewSpacer(), chooseBtn),
	)
}

func (a *App) createRuleCard() *fyne.Container {
	ruleLabel := widget.NewLabel("重命名规则")
	ruleLabel.TextStyle = fyne.TextStyle{Bold: true}

	formatDesc := widget.NewLabel("格式: <父目录><YYYYMMDD>-<序号><扩展名>")
	formatDesc.Wrapping = fyne.TextWrapWord

	exampleDesc := widget.NewLabel("示例:  123/abc.jpg → 12320260323-1.jpg\n        456/img.png → 45620260323-1.png")
	exampleDesc.Wrapping = fyne.TextWrapWord

	return container.NewVBox(
		ruleLabel,
		widget.NewCard("", "", container.NewVBox(formatDesc, exampleDesc)),
	)
}

func (a *App) createAdvancedCard() *fyne.Container {
	expandBtn := widget.NewButton("高级设置 ▼", func() {
	})

	delayLabel := widget.NewLabel("EventDelay (秒)")
	delaySlider := widget.NewSlider(0, 10)
	delaySlider.SetValue(float64(a.cfg.EventDelay))
	delayValue := widget.NewLabel("3s")
	delaySlider.OnChanged = func(val float64) {
		a.cfg.EventDelay = int(val)
		delayValue.SetText(strconv.Itoa(a.cfg.EventDelay) + "s")
	}

	checksLabel := widget.NewLabel("FileStableChecks")
	checksEntry := widget.NewEntry()
	checksEntry.SetText("4")

	intervalLabel := widget.NewLabel("FileStableInterval (秒)")
	intervalSlider := widget.NewSlider(0, 5)
	intervalSlider.SetValue(float64(a.cfg.FileStableInterval))
	intervalValue := widget.NewLabel("1s")
	intervalSlider.OnChanged = func(val float64) {
		a.cfg.FileStableInterval = int(val)
		intervalValue.SetText(strconv.Itoa(a.cfg.FileStableInterval) + "s")
	}

	advancedContent := container.NewVBox(
		container.NewHBox(delayLabel, delaySlider, delayValue),
		container.NewHBox(checksLabel, checksEntry),
		container.NewHBox(intervalLabel, intervalSlider, intervalValue),
	)

	card := widget.NewCard("", "", advancedContent)

	return container.NewVBox(expandBtn, card)
}

func (a *App) createLogCard() *fyne.Container {
	logLabel := widget.NewLabel("实时日志")
	logLabel.TextStyle = fyne.TextStyle{Bold: true}

	a.logList = widget.NewList(
		func() int { return len(a.logEntries) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			entry := a.logEntries[id]
			obj.(*widget.Label).SetText(entry.Time + "  [" + entry.Type + "]  " + entry.Message)
		},
	)

	clearBtn := widget.NewButton("清空", func() {
		a.logEntries = []LogEntry{}
		a.logList.Refresh()
	})

	return container.NewBorder(
		container.NewHBox(logLabel, layout.NewSpacer(), clearBtn),
		nil, nil, nil,
		a.logList,
	)
}

func (a *App) startService() {
	a.statusLabel.SetText("状态: ● 运行中")
	a.addLog("服务", "已启动")
}

func (a *App) stopService() {
	a.statusLabel.SetText("状态: ● 已停止")
	a.addLog("服务", "已停止")
}

func (a *App) restartService() {
	a.statusLabel.SetText("状态: ● 运行中")
	a.addLog("服务", "已重启")
}

func (a *App) addLog(logType, message string) {
	entry := LogEntry{
		Time:    "00:00:00",
		Type:    logType,
		Message: message,
	}
	a.logEntries = append(a.logEntries, entry)
	a.logList.Refresh()
}

type RenamerService struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRenamerService(cfg Config) (*RenamerService, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &RenamerService{ctx: ctx, cancel: cancel}, nil
}

func (s *RenamerService) Close() error {
	s.cancel()
	return nil
}
