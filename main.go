//go:build windows

// AirPods Battery — a Windows system-tray app.
//
// AirPods continuously broadcast a Bluetooth LE advertisement (Apple's
// "proximity pairing" message) carrying the battery level of each bud and the
// case. Windows doesn't show it, but any app listening for BLE adverts can
// read it. This app does that and shows the result in the system tray.
package main

import (
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"tinygo.org/x/bluetooth"
)

const (
	appName     = "AirPods Battery"
	runKeyPath  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runKeyValue = "AirPodsBattery"
)

var (
	dataDir     = filepath.Join(os.Getenv("APPDATA"), "AirPodsBattery")
	historyPath = filepath.Join(dataDir, "history.csv")
)

func main() {
	if !singleInstance() {
		return
	}
	state := NewState(PickWindow)
	go scan(state)
	systray.Run(func() { onReady(state) }, func() {})
}

// ---------------------------------------------------------------- scanning

func scan(state *State) {
	adapter := bluetooth.DefaultAdapter
	for {
		err := adapter.Enable()
		if err == nil {
			err = adapter.Scan(func(_ *bluetooth.Adapter, res bluetooth.ScanResult) {
				if res.RSSI < RSSIMin {
					return
				}
				for _, m := range res.ManufacturerData() {
					if m.CompanyID != AppleCompanyID {
						continue
					}
					if r, ok := Parse(m.Data); ok && r.HasBuds() {
						state.Add(r, res.RSSI, time.Now())
					}
				}
			})
		}
		if err == nil {
			err = errors.New("scan stopped")
		}
		state.SetError(err) // Bluetooth off, adapter busy, etc.
		time.Sleep(10 * time.Second)
	}
}

// ---------------------------------------------------------------- tray

func onReady(state *State) {
	systray.SetIcon(MakeIcon("-", colGrey))
	systray.SetTitle(appName)
	systray.SetTooltip(appName + " — searching…")

	header := systray.AddMenuItem("Searching for AirPods…", "")
	header.Disable()
	left := systray.AddMenuItem("Left: —", "")
	left.Disable()
	right := systray.AddMenuItem("Right: —", "")
	right.Disable()
	caseItem := systray.AddMenuItem("Case: —", "")
	caseItem.Disable()
	systray.AddSeparator()
	history := systray.AddMenuItem("Open battery history", "")
	startup := systray.AddMenuItemCheckbox("Run at Windows startup", "", startupEnabled())
	quit := systray.AddMenuItem("Quit", "")

	go func() {
		for {
			select {
			case <-history.ClickedCh:
				if _, err := os.Stat(historyPath); err == nil {
					_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", historyPath).Start()
				} else {
					notify(appName, "No readings logged yet.")
				}
			case <-startup.ClickedCh:
				if err := setStartup(!startup.Checked()); err != nil {
					notify(appName, "Couldn't change the startup setting: "+err.Error())
				}
				if startupEnabled() {
					startup.Check()
				} else {
					startup.Uncheck()
				}
			case <-quit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go func() {
		armed := map[string]bool{"Left": true, "Right": true}
		var lastLog time.Time
		var lastIcon string
		for {
			r, t, err := state.Snapshot()
			now := time.Now()
			stale := r == nil || now.Sub(t) > StaleAfter

			var text, tip string
			low := -1
			if r != nil {
				low = r.LowestBud()
				text = "-"
				if low >= 0 {
					text = strconv.Itoa(low)
				}
				tip = fmt.Sprintf("%s\nL %s  R %s  Case %s", r.Model,
					fmtLevel(r.Left, r.LeftCharging), fmtLevel(r.Right, r.RightCharging),
					fmtLevel(r.Case, r.CaseCharging))
				if stale {
					tip += "\nLast seen " + ago(now.Sub(t))
				}
				header.SetTitle(r.Model + " · " + ago(now.Sub(t)))
				left.SetTitle("Left: " + fmtLevel(r.Left, r.LeftCharging))
				right.SetTitle("Right: " + fmtLevel(r.Right, r.RightCharging))
				caseItem.SetTitle("Case: " + fmtLevel(r.Case, r.CaseCharging))
			} else {
				text = "-"
				if err != nil {
					tip = appName + " — Bluetooth error"
					header.SetTitle("Bluetooth error: " + truncate(err.Error(), 60))
				} else {
					tip = appName + " — searching…"
				}
			}

			col := ColorFor(low, stale)
			key := fmt.Sprint(text, col, tip)
			if key != lastIcon {
				systray.SetIcon(MakeIcon(text, col))
				systray.SetTooltip(truncate(tip, 127))
				lastIcon = key
			}

			if r != nil && !stale {
				for _, side := range []struct {
					name     string
					level    int
					charging bool
				}{{"Left", r.Left, r.LeftCharging}, {"Right", r.Right, r.RightCharging}} {
					if side.level < 0 {
						continue
					}
					if armed[side.name] && side.level < LowBattery && !side.charging {
						notify("AirPods battery low", fmt.Sprintf("%s AirPod is at %d%%.", side.name, side.level))
						armed[side.name] = false
					} else if side.level >= LowReset || side.charging {
						armed[side.name] = true
					}
				}
				if now.Sub(lastLog) >= LogEvery {
					_ = logReading(*r, now)
					lastLog = now
				}
			}
			time.Sleep(Refresh)
		}
	}()
}

// ---------------------------------------------------------------- helpers

func fmtLevel(v int, charging bool) string {
	if v < 0 {
		return "—"
	}
	s := fmt.Sprintf("%d%%", v)
	if charging {
		s += " ⚡"
	}
	return s
}

func ago(d time.Duration) string {
	switch {
	case d < 90*time.Second:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func logReading(r Reading, now time.Time) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	_, statErr := os.Stat(historyPath)
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if os.IsNotExist(statErr) {
		_ = w.Write([]string{"timestamp", "model", "left", "right", "case",
			"left_charging", "right_charging", "case_charging"})
	}
	level := func(v int) string {
		if v < 0 {
			return ""
		}
		return strconv.Itoa(v)
	}
	_ = w.Write([]string{now.Format("2006-01-02T15:04:05"), r.Model,
		level(r.Left), level(r.Right), level(r.Case),
		strconv.FormatBool(r.LeftCharging), strconv.FormatBool(r.RightCharging),
		strconv.FormatBool(r.CaseCharging)})
	w.Flush()
	return w.Error()
}

func startupEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runKeyValue)
	return err == nil
}

func setStartup(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		return k.DeleteValue(runKeyValue)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runKeyValue, `"`+exe+`"`)
}

func singleInstance() bool {
	name, _ := windows.UTF16PtrFromString("AirPodsBatteryTrayApp")
	_, err := windows.CreateMutex(nil, false, name)
	return !errors.Is(err, windows.ERROR_ALREADY_EXISTS)
}

// notify shows a Windows toast notification. It goes through Windows
// PowerShell's built-in notifier so no app registration is needed.
func notify(title, message string) {
	esc := func(s string) string {
		s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
		return strings.ReplaceAll(s, "'", "''")
	}
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
$x = New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>` + esc(title) + `</text><text>` + esc(message) + `</text></binding></visual></toast>')
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show([Windows.UI.Notifications.ToastNotification]::new($x))`

	u := windows.StringToUTF16(script)
	b := make([]byte, 0, len(u)*2)
	for _, c := range u[:len(u)-1] { // drop trailing NUL
		b = append(b, byte(c), byte(c>>8))
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
