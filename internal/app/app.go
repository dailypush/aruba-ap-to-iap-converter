package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/config"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/converter"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/inventory"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/platform"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/station"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/version"
)

func Run() {
	const appTitle = "Aruba AP to IAP Converter"
	const appSubtitle = "AP-325 Campus-to-Instant conversion station"

	fyneApp := app.NewWithID("com.local.iap325-converter")
	win := fyneApp.NewWindow(appTitle)
	win.Resize(fyne.NewSize(1100, 720))

	cfg := config.Load()
	rootDir, _ := os.Getwd()
	logDir := filepath.Join(rootDir, "logs")

	firmwareEntry := widget.NewEntry()
	firmwareEntry.SetText(cfg.FirmwarePath)
	serialEntry := widget.NewSelectEntry(nil)
	serialEntry.SetText(cfg.SerialPort)
	ifaceEntry := widget.NewSelectEntry(nil)
	ifaceEntry.SetText(cfg.Interface)
	countryEntry := widget.NewEntry()
	countryEntry.SetText(cfg.Country)
	serverEntry := widget.NewEntry()
	serverEntry.SetText(cfg.ServerIP)
	apEntry := widget.NewEntry()
	apEntry.SetText(cfg.APBootIP)
	netmaskEntry := widget.NewEntry()
	netmaskEntry.SetText(cfg.APNetmask)
	expectedEntry := widget.NewEntry()
	alreadyAPBoot := widget.NewCheck("AP is already at apboot> (I verified 235 MB DRAM)", nil)
	enableDHCP := widget.NewCheck("Run direct-connect DHCP helper", nil)
	batchMode := widget.NewCheck("Batch mode: re-enable for next AP after success", nil)
	checksum := widget.NewCheck("I compared the SHA256 with the official Aruba checksum", nil)
	status := widget.NewLabel("Select firmware and confirm checksum.")
	result := widget.NewLabel("")
	progressText := widget.NewLabel("Idle")
	progressBar := widget.NewProgressBar()
	logBox := widget.NewMultiLineEntry()
	logBox.Wrapping = fyne.TextWrapWord
	logBox.SetMinRowsVisible(10)
	logBox.Disable()

	var fwOK bool
	var running bool
	var updateConvertState func()
	session, sessionErr := newSessionLog(logDir)
	sessionPath := "unavailable"
	if session != nil {
		sessionPath = session.Path()
		defer session.Close()
	}
	logLocation := widget.NewLabel("Session log: " + sessionPath)
	logLocation.Wrapping = fyne.TextWrapWord
	if sessionErr != nil {
		logLocation.SetText("Session logging unavailable: " + sessionErr.Error())
	}
	var logFailure sync.Once
	var pendingLog logBuffer
	appendLog := func(s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		if session != nil {
			if err := session.Write(s); err != nil {
				logFailure.Do(func() {
					fyne.Do(func() {
						logLocation.SetText("Session log write failed: " + err.Error())
						dialog.ShowError(fmt.Errorf("session log could not be written: %w", err), win)
					})
				})
			}
		}
		if pendingLog.append(strings.TrimRight(s, "\n") + "\n") {
			time.AfterFunc(100*time.Millisecond, func() {
				fyne.Do(func() {
					logBox.SetText(logTail(logBox.Text + pendingLog.drain()))
					logBox.CursorRow = strings.Count(logBox.Text, "\n")
					logBox.CursorColumn = 0
					logBox.Refresh()
				})
			})
		}
	}
	showError := func(err error) {
		appendLog("[ERROR] " + err.Error())
		dialog.ShowError(err, win)
	}
	appendLog("App started: " + version.Display() + " built " + version.BuiltAt)
	appendLog("Working directory: " + rootDir + "; session log: " + sessionPath)
	showAbout := func() {
		dialog.ShowInformation("About "+appTitle, strings.Join([]string{
			appTitle,
			appSubtitle,
			"Version: " + version.Display(),
			"Built: " + version.BuiltAt,
			"",
			"One-click Aruba AP-325 Campus-to-Instant conversion station.",
			"",
			"Workflow:",
			"- prepare direct Ethernet and TFTP",
			"- catch APBoot over serial",
			"- program country inventory",
			"- flash both Instant image partitions",
			"- factory reset and verify first boot",
			"- probe WebUI/default IP",
			"",
			"Firmware images are user-selected and not bundled.",
			"CSV inventory and conversion results are written to logs/.",
		}, "\n"), win)
	}
	dumpDebug := func() {
		path, summary, err := writeDebugDump(rootDir, logDir, appTitle, appSubtitle, map[string]string{
			"app_version":     version.Version,
			"session_log":     sessionPath,
			"app_commit":      version.Commit,
			"app_built_at":    version.BuiltAt,
			"firmware":        firmwareEntry.Text,
			"serial_port":     serialEntry.Text,
			"interface":       ifaceEntry.Text,
			"country":         countryEntry.Text,
			"server_ip":       serverEntry.Text,
			"apboot_ip":       apEntry.Text,
			"apboot_netmask":  netmaskEntry.Text,
			"expected_serial": expectedEntry.Text,
			"resume":          fmt.Sprintf("%t", alreadyAPBoot.Checked),
			"dhcp_helper":     fmt.Sprintf("%t", enableDHCP.Checked),
			"batch_mode":      fmt.Sprintf("%t", batchMode.Checked),
			"firmware_ok":     fmt.Sprintf("%t", fwOK),
			"running":         fmt.Sprintf("%t", running),
			"status":          status.Text,
			"progress":        progressText.Text,
		})
		if err != nil {
			showError(err)
			return
		}
		appendLog("[OK] debug dump written: " + path)
		content := widget.NewMultiLineEntry()
		content.SetText(summary + "\n\nDebug dump:\n" + path)
		content.SetMinRowsVisible(14)
		dialog.ShowCustom("Debug Dump", "OK", container.NewPadded(content), win)
	}
	setProgress := func(stage string, current, total int, message string) {
		fyne.Do(func() {
			if total <= 0 {
				return
			}
			progressBar.SetValue(float64(current) / float64(total))
			progressText.SetText(fmt.Sprintf("Step %d/%d: %s", current, total, message))
			status.SetText(stage + ": " + message)
		})
	}
	var validatedFirmware firmware.Info
	var validationGeneration uint64
	var validationTimer *time.Timer
	validateFirmware := func() {
		validationGeneration++
		generation := validationGeneration
		path := strings.TrimSpace(firmwareEntry.Text)
		fwOK = false
		validatedFirmware = firmware.Info{}
		checksum.SetChecked(false)
		checksum.Disable()
		status.SetText("Checking firmware...")
		if updateConvertState != nil {
			updateConvertState()
		}
		if validationTimer != nil {
			validationTimer.Stop()
		}
		validationTimer = time.AfterFunc(350*time.Millisecond, func() {
			info, err := firmware.Validate(path)
			fyne.Do(func() {
				if generation != validationGeneration {
					return
				}
				if err != nil {
					status.SetText("Firmware invalid: " + err.Error())
				} else {
					validatedFirmware = info
					fwOK = true
					checksum.Enable()
					status.SetText(fmt.Sprintf("Firmware OK: %s, version %s, SHA256 %s", info.Name, info.Version, info.SHA256))
				}
				if updateConvertState != nil {
					updateConvertState()
				}
			})
		})
	}
	firmwareEntry.OnChanged = func(_ string) { validateFirmware() }

	pickFirmware := widget.NewButton("Choose Firmware", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				showError(err)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			_ = r.Close()
			if firmwareEntry.Text == path {
				validateFirmware()
			} else {
				firmwareEntry.SetText(path)
			}
		}, win)
	})

	currentConfig := func() config.Config {
		return config.Config{
			FirmwarePath: strings.TrimSpace(firmwareEntry.Text),
			SerialPort:   strings.TrimSpace(serialEntry.Text),
			Interface:    strings.TrimSpace(ifaceEntry.Text),
			Country:      strings.ToUpper(strings.TrimSpace(countryEntry.Text)),
			ServerIP:     strings.TrimSpace(serverEntry.Text),
			APBootIP:     strings.TrimSpace(apEntry.Text),
			APNetmask:    strings.TrimSpace(netmaskEntry.Text),
		}
	}
	var refreshAdapters *widget.Button
	refreshAdapters = widget.NewButton("Refresh adapters", func() {
		refreshAdapters.Disable()
		go func() {
			ports, interfaces, err := platform.DiscoverAdapters()
			fyne.Do(func() {
				if !running {
					serialEntry.SetOptions(ports)
					ifaceEntry.SetOptions(interfaces)
					refreshAdapters.Enable()
				}
				if err != nil {
					appendLog("Adapter discovery: " + err.Error())
				}
			})
		}()
	})
	setInputsEnabled := func(enabled bool) {
		for _, entry := range []fyne.Disableable{firmwareEntry, serialEntry, ifaceEntry, countryEntry, serverEntry, apEntry, netmaskEntry, expectedEntry} {
			if enabled {
				entry.Enable()
			} else {
				entry.Disable()
			}
		}
		for _, check := range []*widget.Check{checksum, alreadyAPBoot, enableDHCP, batchMode} {
			if enabled {
				check.Enable()
			} else {
				check.Disable()
			}
		}
		if enabled {
			pickFirmware.Enable()
			refreshAdapters.Enable()
		} else {
			pickFirmware.Disable()
			refreshAdapters.Disable()
		}
	}

	convertButton := widget.NewButton("Convert AP", nil)
	convertButton.Importance = widget.HighImportance
	debugButton := widget.NewButton("Dump Debug Info", dumpDebug)
	aboutButton := widget.NewButton("About", showAbout)
	win.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("Tools",
			fyne.NewMenuItem("Dump Debug Info", dumpDebug),
		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About "+appTitle, showAbout),
		),
	))
	updateConvertState = func() {
		if running || !fwOK || !checksum.Checked {
			convertButton.Disable()
			return
		}
		convertButton.Enable()
	}
	checksum.OnChanged = func(bool) { updateConvertState() }
	convertButton.OnTapped = func() {
		if running {
			return
		}
		if !fwOK {
			showError(fmt.Errorf("select a valid Aruba Instant Hercules 6.x image first"))
			return
		}
		if !checksum.Checked {
			showError(fmt.Errorf("confirm the SHA256 checksum before conversion"))
			return
		}
		runConfig := currentConfig()
		expectedSerial := strings.TrimSpace(expectedEntry.Text)
		if err := runConfig.Validate(); err != nil {
			showError(err)
			return
		}
		if err := config.ValidateSerial(expectedSerial); err != nil {
			showError(err)
			return
		}
		approvedSHA := validatedFirmware.SHA256
		resume, dhcp, batch := alreadyAPBoot.Checked, enableDHCP.Checked, batchMode.Checked
		if err := config.Save(runConfig); err != nil {
			showError(fmt.Errorf("save settings: %w", err))
			return
		}
		running = true
		appendLog(fmt.Sprintf("Starting conversion: interface=%s server=%s ap=%s serial_port=%s expected_serial=%s dhcp=%t", runConfig.Interface, runConfig.ServerIP, runConfig.APBootIP, runConfig.SerialPort, expectedSerial, dhcp))
		setInputsEnabled(false)
		convertButton.Disable()
		convertButton.SetText("Convert AP")
		logBox.SetText("")
		result.SetText("")
		progressBar.SetValue(0)
		progressText.SetText("Starting...")
		status.SetText("Preparing station...")

		go func() {
			defer func() {
				fyne.Do(func() {
					running = false
					setInputsEnabled(true)
					updateConvertState()
				})
			}()

			setup := platform.Current()
			executable, _ := os.Executable()
			setProgress("station-setup", 1, 12, "Preparing direct-connect station")
			prep, err := station.Prepare(station.Request{
				Interface:      runConfig.Interface,
				ServerIP:       runConfig.ServerIP,
				APBootIP:       runConfig.APBootIP,
				Netmask:        runConfig.APNetmask,
				FirmwarePath:   runConfig.FirmwarePath,
				SerialPort:     runConfig.SerialPort,
				RootDir:        rootDir,
				Executable:     executable,
				EnableDHCP:     dhcp,
				ApprovedSHA256: approvedSHA,
			}, setup, appendLog)
			if err != nil {
				fyne.Do(func() {
					status.SetText("Station setup failed.")
					result.SetText("Error: " + err.Error() + "\nSession log: " + sessionPath)
					showError(err)
				})
				return
			}

			fyne.Do(func() { status.SetText("Waiting for AP power-cycle/APBoot...") })
			engine := converter.Engine{
				Store:    inventory.New(logDir),
				Log:      appendLog,
				Progress: setProgress,
			}
			res, err := engine.Run(context.Background(), converter.Request{
				Port:           runConfig.SerialPort,
				Baud:           9600,
				Country:        runConfig.Country,
				ExpectedSerial: expectedSerial,
				ServerIP:       runConfig.ServerIP,
				APBootIP:       runConfig.APBootIP,
				APBootNetmask:  runConfig.APNetmask,
				TFTPImage:      prep.TFTPImage,
				Firmware:       prep.Firmware,
				LogDir:         prep.LogDir,
				BootTimeout:    180 * time.Second,
				AlreadyAPBoot:  resume,
			})
			if cleanupErr := prep.Cleanup(); cleanupErr != nil {
				res.Warning = strings.TrimSpace(res.Warning + " " + cleanupErr.Error())
				res.Message += "; " + cleanupErr.Error()
				appendLog(cleanupErr.Error())
			}
			fyne.Do(func() {
				if err != nil {
					status.SetText("Conversion stopped: " + res.Stage)
					result.SetText(fmt.Sprintf("Stage: %s\nMessage: %s\nLog: %s", res.Stage, res.Message, res.LogPath))
					showError(err)
					return
				}
				progressBar.SetValue(1)
				progressText.SetText("Step 12/12: Conversion complete")
				status.SetText("Conversion succeeded.")
				result.SetText(fmt.Sprintf("Serial: %s\nWired MAC: %s\nVersion: %s\nLog: %s\nWebUI: %s", res.Serial, res.WiredMAC, res.ImageVersion, res.LogPath, res.WebURL))
				if res.Warning != "" {
					status.SetText("Converted; attention needed.")
					appendLog("[WARNING] " + res.Warning)
					dialog.ShowInformation("Conversion warning", res.Warning, win)
				}
				if batch && res.Warning == "" {
					alreadyAPBoot.SetChecked(false)
					expectedEntry.SetText("")
					status.SetText("Batch ready: connect the next AP, then click Convert Next AP.")
					convertButton.SetText("Convert Next AP")
				}
			})
		}()
	}
	updateConvertState()
	// Recheck on launch without blocking the event loop.
	validateFirmware()
	refreshAdapters.OnTapped()
	win.SetCloseIntercept(func() {
		if running {
			dialog.ShowInformation("Conversion in progress", "Wait for conversion to finish before closing. Interrupting a firmware write may leave the AP unusable.", win)
			return
		}
		validationGeneration++
		if validationTimer != nil {
			validationTimer.Stop()
		}
		win.Close()
	})

	requiredForm := widget.NewForm(
		widget.NewFormItem("Firmware", container.NewBorder(nil, nil, nil, pickFirmware, firmwareEntry)),
		widget.NewFormItem("Serial Port", serialEntry),
		widget.NewFormItem("Interface", container.NewBorder(nil, nil, nil, refreshAdapters, ifaceEntry)),
		widget.NewFormItem("Country", countryEntry),
	)
	networkForm := widget.NewForm(
		widget.NewFormItem("Server IP", serverEntry),
		widget.NewFormItem("APBoot IP", apEntry),
		widget.NewFormItem("APBoot Netmask", netmaskEntry),
	)
	advancedForm := widget.NewForm(
		widget.NewFormItem("Expected Serial (optional safety check)", expectedEntry),
		widget.NewFormItem("Resume", alreadyAPBoot),
		widget.NewFormItem("DHCP Helper", enableDHCP),
		widget.NewFormItem("Batch", batchMode),
	)
	advanced := widget.NewAccordion(
		widget.NewAccordionItem("Network Defaults", networkForm),
		widget.NewAccordionItem("Advanced Options", advancedForm),
	)
	status.Wrapping = fyne.TextWrapWord
	result.Wrapping = fyne.TextWrapWord
	progressText.Wrapping = fyne.TextWrapWord

	actionPanel := container.NewVBox(
		checksum,
		container.NewHBox(convertButton, debugButton, aboutButton),
		progressText,
		progressBar,
		status,
	)
	setupCard := widget.NewCard("Setup", "Required fields for one conversion station", container.NewVBox(
		requiredForm,
		advanced,
	))
	actionCard := widget.NewCard("Run", "Confirm checksum, then start conversion", actionPanel)
	resultCard := widget.NewCard("Result", "Latest conversion output", result)
	topPanel := container.NewGridWithColumns(
		2,
		container.NewVBox(setupCard),
		container.NewVBox(actionCard, resultCard),
	)

	logPanel := container.NewBorder(
		container.NewVBox(widget.NewLabel("Live activity log"), logLocation),
		nil,
		nil,
		nil,
		container.NewPadded(logBox),
	)
	mainSplit := container.NewVSplit(container.NewVScroll(topPanel), logPanel)
	mainSplit.SetOffset(0.52)

	win.SetContent(container.NewBorder(
		widget.NewRichTextFromMarkdown("## "+appTitle+"\n"+appSubtitle),
		nil,
		nil,
		nil,
		mainSplit,
	))
	win.ShowAndRun()
}

func writeDebugDump(rootDir, logDir, appTitle, appSubtitle string, settings map[string]string) (string, string, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", "", err
	}
	path := filepath.Join(logDir, "debug-dump-"+time.Now().Format("20060102-150405")+".txt")
	var b strings.Builder
	writeSection(&b, appTitle+" Debug Dump")
	b.WriteString("timestamp: " + time.Now().Format(time.RFC3339) + "\n")
	b.WriteString("app_title: " + appTitle + "\n")
	b.WriteString("app_subtitle: " + appSubtitle + "\n")
	b.WriteString("root_dir: " + rootDir + "\n")
	b.WriteString("log_dir: " + logDir + "\n")

	writeSection(&b, "Current UI Settings")
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k + ": " + settings[k] + "\n")
	}

	writePlatformDebugCommands(&b, rootDir, settings)

	writeFileTail(&b, filepath.Join(logDir, "conversion-results.csv"), 20)
	writeFileTail(&b, filepath.Join(logDir, "ap-inventory.csv"), 20)
	writeFileTail(&b, filepath.Join(logDir, "dhcp-helper.log"), 80)
	writeFileTail(&b, filepath.Join(logDir, "tftp-helper.log"), 80)
	for _, file := range newestLogs(logDir, "app-session-", 2) {
		writeFileTail(&b, file, 120)
	}
	for _, file := range newestLogs(logDir, "auto-convert-", 3) {
		writeFileTail(&b, file, 120)
	}
	for _, file := range newestLogs(logDir, "instant-verify-", 2) {
		writeFileTail(&b, file, 80)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", "", err
	}
	summary := strings.Join([]string{
		"Debug dump created.",
		"",
		"Included:",
		"- current GUI settings",
		"- interface status",
		"- serial-port ownership check",
		"- TFTP UDP/69 listener check",
		"- ARP table",
		"- TFTP image directory listing",
		"- CSV tails",
		"- DHCP/TFTP helper log tails",
		"- app session logs, including setup errors",
		"- newest conversion/verify log tails",
	}, "\n")
	return path, summary, nil
}

func writePlatformDebugCommands(b *strings.Builder, rootDir string, settings map[string]string) {
	switch runtime.GOOS {
	case "windows":
		writeCommand(b, "ipconfig", "ipconfig", "/all")
		writeCommand(b, "interface addresses", "netsh", "interface", "ipv4", "show", "addresses", settings["interface"])
		writeCommand(b, "UDP listeners", "netstat", "-ano", "-p", "udp")
		writeCommand(b, "ARP table", "arp", "-a")
		writeCommand(b, "serial port mode", "mode", settings["serial_port"])
		writeCommand(b, "TFTP aruba directory", "cmd", "/C", "dir", filepath.Join(rootDir, "tftpboot", "aruba"))
	case "darwin":
		writeCommand(b, "ifconfig "+settings["interface"], "ifconfig", settings["interface"])
		writeCommand(b, "lsof serial port", "lsof", settings["serial_port"])
		writeCommand(b, "UDP/69 listeners", "lsof", "-nP", "-iUDP:69")
		writeCommand(b, "ARP table", "arp", "-an")
		writeCommand(b, "TFTP aruba directory", "ls", "-la", "/private/tftpboot/aruba")
	default:
		writeCommand(b, "interface address", "ip", "addr", "show", settings["interface"])
		writeCommand(b, "serial-port ownership", "lsof", settings["serial_port"])
		writeCommand(b, "UDP/69 listeners", "ss", "-lunp")
		writeCommand(b, "ARP table", "arp", "-an")
		writeCommand(b, "TFTP aruba directory", "ls", "-la", filepath.Join(rootDir, "tftpboot", "aruba"))
	}
}

func writeSection(b *strings.Builder, title string) {
	b.WriteString("\n==== " + title + " ====\n")
}

func writeCommand(b *strings.Builder, title string, name string, args ...string) {
	writeSection(b, title)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.WriteString("error: " + err.Error() + "\n")
	}
	if len(out) == 0 {
		b.WriteString("(no output)\n")
		return
	}
	b.Write(out)
	if !strings.HasSuffix(string(out), "\n") {
		b.WriteString("\n")
	}
}

func writeFileTail(b *strings.Builder, path string, maxLines int) {
	writeSection(b, "tail "+path)
	data, err := os.ReadFile(path)
	if err != nil {
		b.WriteString("error: " + err.Error() + "\n")
		return
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}
	b.WriteString(strings.Join(lines[start:], "\n"))
	if !strings.HasSuffix(string(data), "\n") {
		b.WriteString("\n")
	}
}

func newestLogs(logDir, prefix string, limit int) []string {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	type candidate struct {
		path string
		mod  time.Time
	}
	var files []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, candidate{
			path: filepath.Join(logDir, entry.Name()),
			mod:  info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.After(files[j].mod)
	})
	out := make([]string, 0, limit)
	for i, file := range files {
		if i >= limit {
			break
		}
		out = append(out, file.path)
	}
	return out
}
