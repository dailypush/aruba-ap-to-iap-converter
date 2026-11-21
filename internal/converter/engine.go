package converter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/config"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/inventory"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/parser"
	serialclient "github.com/dailypush/aruba-ap-to-iap-converter/internal/serial"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/version"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/webprobe"
)

type Request struct {
	Port           string
	Baud           int
	Country        string
	ExpectedSerial string
	ServerIP       string
	APBootIP       string
	APBootNetmask  string
	TFTPImage      string
	Firmware       firmware.Info
	LogDir         string
	BootTimeout    time.Duration
	AlreadyAPBoot  bool
}

type Result struct {
	Serial       string
	WiredMAC     string
	ImageVersion string
	Status       string
	Stage        string
	Message      string
	LogPath      string
	WebURL       string
	Warning      string
}

type Console interface {
	Close() error
	SendLine(string) error
	ReadUntil(*regexp.Regexp, time.Duration) (string, error)
	Drain(time.Duration) string
	CatchAPBoot(time.Duration) (string, error)
	ExpectAPBoot(time.Duration) (string, error)
}

type Engine struct {
	openConsole func(string, int, func(string)) (Console, error)
	Store       inventory.Store
	Log         func(string)
	Progress    func(stage string, current, total int, message string)
}

func (e Engine) Run(ctx context.Context, req Request) (Result, error) {
	if err := validateRequest(req); err != nil {
		return Result{Status: "FAILED", Stage: "preflight", Message: err.Error()}, err
	}
	if req.Baud == 0 {
		req.Baud = 9600
	}
	if req.BootTimeout == 0 {
		req.BootTimeout = 180 * time.Second
	}
	if err := os.MkdirAll(req.LogDir, 0755); err != nil {
		return Result{}, err
	}
	f, err := os.CreateTemp(req.LogDir, "auto-convert-"+time.Now().Format("20060102-150405")+"-*.log")
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	logPath := f.Name()
	var transcriptErr error

	atLineStart := true
	writeLog := func(s string) {
		if s == "" {
			return
		}
		if _, err := f.WriteString(s); err != nil {
			transcriptErr = err
		}
		atLineStart = strings.HasSuffix(s, "\n")
		if e.Log != nil {
			e.Log(s)
		}
	}
	log := func(s string) {
		if !atLineStart {
			s = "\n" + s
		}
		writeLog(strings.TrimRight(s, "\n") + "\n")
	}

	res := Result{Status: "FAILED", Stage: "startup", LogPath: logPath, ImageVersion: req.Firmware.Version}
	progress := func(stage string, current int, message string) {
		if e.Progress != nil {
			e.Progress(stage, current, 12, message)
		}
	}
	writeResult := func(status, stage, message string) {
		res.Status = status
		res.Stage = stage
		res.Message = message
		persistErr := e.Store.AppendResult(inventory.Result{
			Timestamp:    time.Now(),
			Serial:       valueOr(res.Serial, "UNKNOWN"),
			WiredMAC:     valueOr(res.WiredMAC, "UNKNOWN"),
			Country:      strings.ToUpper(req.Country),
			ImageVersion: req.Firmware.Version,
			Status:       status,
			Stage:        stage,
			Message:      message,
			LogPath:      logPath,
		})
		if err := errors.Join(transcriptErr, persistErr); err != nil {
			res.Warning = "Recording incomplete: " + err.Error() + ". Preserve the AP serial and conversion details before continuing."
			res.Message += "; " + res.Warning
			log(res.Warning)
		}
	}

	open := e.openConsole
	if open == nil {
		open = func(port string, baud int, log func(string)) (Console, error) {
			return serialclient.Open(port, baud, log)
		}
	}
	client, err := open(req.Port, req.Baud, writeLog)
	if err != nil {
		writeResult("FAILED", "serial", err.Error())
		return res, err
	}
	defer client.Close()

	stage := "catch-apboot"
	if req.AlreadyAPBoot {
		progress(stage, 2, "Checking for the APBoot prompt")
	} else {
		progress(stage, 2, "Waiting for APBoot. Power-cycle the AP now.")
	}
	log("Automated AP-325 conversion starting")
	log("App version: " + version.Display() + " built " + version.BuiltAt)
	if req.AlreadyAPBoot {
		log("Resume mode: expecting AP is already at apboot> after prior banner verification.")
		if _, err := client.ExpectAPBoot(10 * time.Second); err != nil {
			writeResult("FAILED", stage, "Resume mode did not find apboot>")
			return res, err
		}
	} else {
		log("Waiting for APBoot. Power-cycle the AP now.")
		banner, err := client.CatchAPBoot(req.BootTimeout)
		if err != nil {
			writeResult("FAILED", stage, "Timed out waiting for apboot>; power-cycle the AP while this app is waiting")
			return res, err
		}
		if !regexp.MustCompile(`DRAM:\s+235 MB`).MatchString(banner) {
			writeResult("FAILED", stage, "Reached apboot> without observing the 235 MB boot banner; power-cycle while waiting or enable resume mode after manually verifying 235 MB DRAM")
			return res, fmt.Errorf("missing 235 MB boot banner")
		}
	}

	stage = "identify"
	progress(stage, 3, "Reading manufacturing info")
	mfg, err := commandExpect(client, "mfginfo", regexp.MustCompile(`(?is)Card\s+0:\s*System`), 30*time.Second, 3)
	if err != nil {
		writeResult("FAILED", stage, err.Error())
		return res, err
	}
	mi, err := parser.ParseMfgInfo(mfg)
	if err != nil {
		writeResult("FAILED", stage, err.Error())
		return res, err
	}
	res.Serial = mi.SystemSerial
	res.WiredMAC = mi.WiredMAC
	log("Detected system serial: " + res.Serial)
	log("Detected wired MAC: " + res.WiredMAC)
	if req.ExpectedSerial != "" && !strings.EqualFold(req.ExpectedSerial, res.Serial) {
		msg := fmt.Sprintf("Connected AP serial %s does not match expected serial %s", res.Serial, req.ExpectedSerial)
		writeResult("FAILED", stage, msg)
		return res, errors.New(msg)
	}

	stage = "network"
	progress(stage, 4, "Configuring APBoot network")
	for _, cmd := range []string{
		"setenv ipaddr " + req.APBootIP,
		"setenv netmask " + req.APBootNetmask,
		"setenv serverip " + req.ServerIP,
	} {
		out, err := command(client, cmd, 30*time.Second)
		if err != nil || reject(out) {
			writeResult("FAILED", stage, "APBoot reported a failure while running: "+cmd)
			return res, fmt.Errorf("failed: %s", cmd)
		}
	}
	progress(stage, 4, "Checking whether APBoot can reach "+req.ServerIP)
	ping, err := command(client, "ping "+req.ServerIP, 120*time.Second)
	if err != nil || !regexp.MustCompile(`(?i)is alive|host .* alive|bytes from`).MatchString(ping) {
		message := fmt.Sprintf("APBoot could not confirm reachability to %s. Check the direct Ethernet link and the selected Mac adapter before trying again.", req.ServerIP)
		writeResult("FAILED", stage, message)
		return res, errors.New(message)
	}

	stage = "inventory"
	progress(stage, 5, "Programming country inventory")
	ccode := parser.CountryCodeCommand(req.Country, res.Serial)
	for _, cmd := range []string{ccode, "invent -w"} {
		out, err := command(client, cmd, 60*time.Second)
		if err != nil || reject(out) {
			writeResult("FAILED", stage, "APBoot reported a failure while running: "+cmd)
			return res, fmt.Errorf("failed: %s", cmd)
		}
	}

	for _, part := range []string{"0", "1"} {
		stage = "upgrade-partition-" + part
		if part == "0" {
			progress(stage, 6, "Flashing partition 0")
		} else {
			progress(stage, 7, "Flashing partition 1")
		}
		cmd := "upgrade os " + part + " " + req.TFTPImage
		out, err := command(client, cmd, 15*time.Minute)
		if err != nil || reject(out) || !regexp.MustCompile(`(?i)signature verified|checksum.*passed|upgrade.*successful|done`).MatchString(out) {
			writeResult("FAILED", stage, "Partition "+part+" upgrade did not report a verifiable success")
			return res, fmt.Errorf("partition %s upgrade failed", part)
		}
	}

	stage = "verify-images"
	progress(stage, 8, "Verifying both partitions")
	osinfo, err := command(client, "osinfo", 2*time.Minute)
	if err != nil || reject(osinfo) || !parser.VerifyOSInfo(osinfo, req.Firmware.Version) {
		writeResult("FAILED", stage, "osinfo did not show target version "+req.Firmware.Version+" on both partitions")
		return res, fmt.Errorf("osinfo verification failed")
	}

	stage = "factory-reset"
	progress(stage, 9, "Factory reset and save environment")
	for _, cmd := range []string{"factory_reset", "saveenv"} {
		out, err := command(client, cmd, 5*time.Minute)
		if err != nil || reject(out) {
			writeResult("FAILED", stage, "APBoot reported a failure while running: "+cmd)
			return res, fmt.Errorf("failed: %s", cmd)
		}
	}

	stage = "first-boot"
	progress(stage, 10, "Booting target Instant image")
	if err := client.SendLine("boot"); err != nil {
		writeResult("FAILED", stage, err.Error())
		return res, err
	}
	boot, err := client.ReadUntil(regexp.MustCompile(`(?i)ArubaOS Version\s+`+regexp.QuoteMeta(req.Firmware.Version)+`(?:\s|\()|`+panicPattern), 5*time.Minute)
	if err != nil || parser.HasPanic(boot) {
		writeResult("FAILED", stage, "Target image boot verification failed (timeout, serial error, or panic)")
		return res, fmt.Errorf("first boot verification failed")
	}
	log("Target image " + req.Firmware.Version + " is booting.")

	stage = "webui"
	progress(stage, 11, "Waiting for WebUI/default IP")
	postBoot := boot
	if !bootReady.MatchString(postBoot) {
		rest, readErr := client.ReadUntil(bootReadyOrPanic, 3*time.Minute)
		postBoot += rest
		if parser.HasPanic(postBoot) {
			writeResult("FAILED", stage, "Firmware flashed, but the AP reported a panic or reboot during startup")
			return res, fmt.Errorf("post-boot panic or reboot detected")
		}
		if readErr != nil {
			writeResult("UNVERIFIED", stage, "Firmware flashed; startup readiness could not be verified: "+readErr.Error())
			return res, fmt.Errorf("startup readiness: %w", readErr)
		}
	}
	tail, tailErr := client.ReadUntil(regexp.MustCompile(`(?i)`+panicPattern), 2*time.Second)
	postBoot += tail
	if parser.HasPanic(postBoot) {
		writeResult("FAILED", stage, "Firmware flashed, but the AP reported a panic or reboot during startup")
		return res, fmt.Errorf("post-boot panic or reboot detected")
	}
	if tailErr != nil && !errors.Is(tailErr, serialclient.ErrTimeout) {
		writeResult("UNVERIFIED", stage, "Firmware flashed; serial monitoring stopped: "+tailErr.Error())
		return res, fmt.Errorf("post-boot monitoring: %w", tailErr)
	}
	if ip, ok := parser.ParseDefaultIP(postBoot); ok {
		progress(stage, 11, "Probing WebUI at "+ip.String())
		if probed, ok := webprobe.Probe(ctx, ip.String(), ""); ok {
			res.WebURL = probed
			log("WebUI reachable: " + probed)
		} else {
			res.WebURL = "https://" + ip.String() + "/"
			log("WebUI URL candidate: " + res.WebURL)
		}
	}

	if err := f.Sync(); err != nil {
		transcriptErr = err
	}
	writeResult("SUCCESS", "complete", "Target Instant image reached startup readiness")
	progress("complete", 12, "Conversion complete")
	log("SUCCESS: converted AP " + res.Serial + " to Instant Hercules " + req.Firmware.Version)
	return res, nil
}

func command(client Console, cmd string, timeout time.Duration) (string, error) {
	client.Drain(300 * time.Millisecond)
	if err := client.SendLine(cmd); err != nil {
		return "", err
	}
	return client.ReadUntil(regexp.MustCompile(`(?s)apboot\s*> ?$`), timeout)
}

func commandExpect(client Console, cmd string, expected *regexp.Regexp, timeout time.Duration, attempts int) (string, error) {
	var last string
	var lastErr error
	for i := 0; i < attempts; i++ {
		// Read through the prompt in one operation. A single serial read can contain
		// both the expected marker and the prompt; a second read would lose it.
		out, err := command(client, cmd, timeout)
		last = out
		lastErr = err
		if err == nil && expected.MatchString(out) {
			return out, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return last, lastErr
	}
	return last, fmt.Errorf("%s output did not include expected APBoot data", cmd)
}

func reject(output string) bool {
	re := regexp.MustCompile(`(?i)retry count exceeded|timed out|tftp error|not found|unknown command|invalid command|access denied|abort|is not alive|upgrade failed|checksum.*failed|signature.*failed`)
	return re.MatchString(output)
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

const panicPattern = `Kernel panic|panic:|Invalid image format|rebooting in`

var bootReady = regexp.MustCompile(`(?i)Default IP is ready|User:`)
var bootReadyOrPanic = regexp.MustCompile(`(?i)Default IP is ready|User:|` + panicPattern)

func validateRequest(req Request) error {
	if strings.TrimSpace(req.Port) == "" {
		return fmt.Errorf("serial port is required")
	}
	if err := config.ValidateCountry(req.Country); err != nil {
		return err
	}
	if err := config.ValidateSerial(req.ExpectedSerial); err != nil {
		return err
	}
	if err := config.ValidateNetwork(req.ServerIP, req.APBootIP, req.APBootNetmask); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_./-]+$`).MatchString(req.TFTPImage) {
		return fmt.Errorf("TFTP image name contains unsupported characters")
	}
	if req.Firmware.Version == "" {
		return fmt.Errorf("validated firmware version is required")
	}
	return nil
}
