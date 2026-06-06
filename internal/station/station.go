package station

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/chadedwards/iap325-converter/internal/firmware"
	"github.com/chadedwards/iap325-converter/internal/platform"
)

type Request struct {
	Interface    string
	ServerIP     string
	APBootIP     string
	Netmask      string
	FirmwarePath string
	SerialPort   string
	RootDir      string
	Executable   string
	EnableDHCP   bool
}

type Prepared struct {
	Firmware  firmware.Info
	TFTPImage string
	TFTPPath  string
	LogDir    string
}

func Prepare(req Request, setup platform.Setup, log func(string)) (Prepared, error) {
	info, err := firmware.Validate(req.FirmwarePath)
	if err != nil {
		return Prepared{}, err
	}
	log("[OK] firmware validated: " + info.Name)
	log("SHA256: " + info.SHA256)

	logDir := filepath.Join(req.RootDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return Prepared{}, err
	}
	tftpPath := filepath.Join("/private/tftpboot", "aruba", info.Name)
	tftpImage := "aruba/" + info.Name
	if err := setup.EnsureInterfaceAlias(platform.SetupRequest{
		Interface: req.Interface,
		ServerIP:  req.ServerIP,
		Netmask:   req.Netmask,
	}, log); err != nil {
		return Prepared{}, err
	}
	if req.EnableDHCP {
		dhcpLog := filepath.Join(logDir, "dhcp-helper.log")
		if err := setup.EnsureDHCPHelper(platform.SetupRequest{
			Interface: req.Interface,
			ServerIP:  req.ServerIP,
			LeaseIP:   req.APBootIP,
			Netmask:   req.Netmask,
			LogPath:   dhcpLog,
		}, req.Executable, log); err != nil {
			return Prepared{}, err
		}
		log("[OK] DHCP helper requested for " + req.APBootIP)
		log("DHCP helper log: " + dhcpLog)
	}
	if verifySameSHA(req.FirmwarePath, tftpPath) == nil {
		log("[OK] TFTP image already matches local firmware: " + tftpPath)
	} else {
		if err := setup.EnsureTFTPImage(platform.SetupRequest{
			FirmwarePath: req.FirmwarePath,
			TFTPPath:     tftpPath,
		}, log); err != nil {
			return Prepared{}, err
		}
		if err := verifySameSHA(req.FirmwarePath, tftpPath); err != nil {
			return Prepared{}, err
		}
	}
	log("[OK] TFTP image present: " + tftpPath)
	if err := setup.EnsureTFTPService(log); err != nil {
		return Prepared{}, err
	}
	inUse, detail := setup.SerialInUse(req.SerialPort)
	if inUse {
		return Prepared{}, &SerialBusyError{Detail: detail}
	}
	log("[OK] serial port is free: " + req.SerialPort)

	return Prepared{Firmware: info, TFTPImage: tftpImage, TFTPPath: tftpPath, LogDir: logDir}, nil
}

type SerialBusyError struct {
	Detail string
}

func (e *SerialBusyError) Error() string {
	return "serial port is already in use"
}

func verifySameSHA(a, b string) error {
	ha, err := fileSHA(a)
	if err != nil {
		return err
	}
	hb, err := fileSHA(b)
	if err != nil {
		return err
	}
	if ha != hb {
		return os.ErrInvalid
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
