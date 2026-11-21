package station

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/config"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/platform"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/tftphelper"
)

type Request struct {
	Interface      string
	ServerIP       string
	APBootIP       string
	Netmask        string
	FirmwarePath   string
	SerialPort     string
	RootDir        string
	Executable     string
	EnableDHCP     bool
	ApprovedSHA256 string
}

type Prepared struct {
	Firmware  firmware.Info
	TFTPImage string
	TFTPPath  string
	LogDir    string
	Cleanup   func() error
}

func Prepare(req Request, setup platform.Setup, log func(string)) (Prepared, error) {
	if log == nil {
		log = func(string) {}
	}
	if err := config.ValidateNetwork(req.ServerIP, req.APBootIP, req.Netmask); err != nil {
		return Prepared{}, err
	}
	if strings.TrimSpace(req.Interface) == "" || strings.TrimSpace(req.SerialPort) == "" {
		return Prepared{}, fmt.Errorf("interface and serial port are required")
	}
	info, err := firmware.Validate(req.FirmwarePath)
	if err != nil {
		return Prepared{}, err
	}
	if req.ApprovedSHA256 == "" || info.SHA256 != req.ApprovedSHA256 {
		return Prepared{}, fmt.Errorf("firmware changed or checksum was not approved; reselect the image and compare its SHA256 again")
	}
	inUse, detail := setup.SerialInUse(req.SerialPort)
	if inUse {
		return Prepared{}, &SerialBusyError{Detail: detail}
	}
	log("[OK] firmware validated: " + info.Name)
	log("SHA256: " + info.SHA256)

	logDir := filepath.Join(req.RootDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return Prepared{}, err
	}
	tftpPath, tftpImage := setup.TFTPPaths(req.RootDir, info.Name)
	if err := setup.EnsureInterfaceAlias(platform.SetupRequest{
		Interface: req.Interface,
		ServerIP:  req.ServerIP,
		Netmask:   req.Netmask,
	}, log); err != nil {
		return Prepared{}, err
	}

	if verifySHA(tftpPath, info.SHA256) == nil {
		log("[OK] TFTP image already matches local firmware: " + tftpPath)
	} else {
		if err := setup.EnsureTFTPImage(platform.SetupRequest{
			FirmwarePath: req.FirmwarePath,
			TFTPPath:     tftpPath,
			TFTPRoot:     filepath.Dir(filepath.Dir(tftpPath)),
		}, log); err != nil {
			return Prepared{}, err
		}
		if err := verifySHA(tftpPath, info.SHA256); err != nil {
			return Prepared{}, err
		}
	}
	log("[OK] TFTP image present: " + tftpPath)
	tftpRoot := filepath.Dir(filepath.Dir(tftpPath))
	tftpLog := filepath.Join(logDir, "tftp-helper.log")
	if err := setup.EnsureTFTPService(platform.SetupRequest{
		TFTPRoot:       tftpRoot,
		ServerIP:       req.ServerIP,
		TFTPPath:       tftpPath,
		ExpectedSHA256: info.SHA256,
		Executable:     req.Executable,
		LogPath:        tftpLog,
	}, log); err != nil {
		return Prepared{}, err
	}
	verifyAddress := platform.TFTPVerifyAddress(req.ServerIP)
	log("Checking local TFTP service at " + verifyAddress + "; APBoot server IP remains " + req.ServerIP)
	if err := tftphelper.Verify(verifyAddress, tftpImage, info.SHA256, 20*time.Second); err != nil {
		return Prepared{}, fmt.Errorf("local TFTP check at %s failed: %w", verifyAddress, err)
	}
	log("[OK] TFTP download matches the approved firmware")
	log("[OK] serial port is free: " + req.SerialPort)

	cleanup := func() error { return nil }
	if req.EnableDHCP {
		dhcpLog := filepath.Join(logDir, "dhcp-helper.log")
		stop, err := setup.EnsureDHCPHelper(platform.SetupRequest{
			Interface: req.Interface,
			ServerIP:  req.ServerIP,
			LeaseIP:   req.APBootIP,
			Netmask:   req.Netmask,
			LogPath:   dhcpLog,
		}, req.Executable, log)
		if err != nil {
			return Prepared{}, err
		}
		cleanup = stop
		log("[OK] DHCP helper ready for " + req.APBootIP)
		log("DHCP helper log: " + dhcpLog)
	}
	return Prepared{Firmware: info, TFTPImage: tftpImage, TFTPPath: tftpPath, LogDir: logDir, Cleanup: cleanup}, nil
}

type SerialBusyError struct {
	Detail string
}

func (e *SerialBusyError) Error() string {
	return "serial port is already in use"
}

func verifySHA(path, expected string) error {
	actual, err := fileSHA(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("staged firmware checksum does not match the approved image")
	}
	return nil
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
