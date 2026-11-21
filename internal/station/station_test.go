package station

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/platform"
)

type fakeSetup struct {
	root    string
	changes int
}

func (f *fakeSetup) TFTPPaths(_, name string) (string, string) {
	return filepath.Join(f.root, "tftpboot", "aruba", name), "aruba/" + name
}
func (f *fakeSetup) EnsureInterfaceAlias(platform.SetupRequest, func(string)) error {
	f.changes++
	return nil
}
func (f *fakeSetup) EnsureDHCPHelper(platform.SetupRequest, string, func(string)) (func() error, error) {
	f.changes++
	return func() error { return nil }, nil
}
func (f *fakeSetup) EnsureTFTPService(platform.SetupRequest, func(string)) error {
	f.changes++
	return nil
}
func (f *fakeSetup) SerialInUse(string) (bool, string) { return false, "" }
func (f *fakeSetup) EnsureTFTPImage(r platform.SetupRequest, _ func(string)) error {
	f.changes++
	if err := os.MkdirAll(filepath.Dir(r.TFTPPath), 0755); err != nil {
		return err
	}
	// Simulate a source file replaced after initial validation and copied to TFTP.
	if err := os.WriteFile(r.FirmwarePath, []byte("changed firmware"), 0644); err != nil {
		return err
	}
	return os.WriteFile(r.TFTPPath, []byte("changed firmware"), 0644)
}
func TestChecksumApprovalBeforeStationChanges(t *testing.T) {
	for _, digest := range []string{"", "wrong"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "ArubaInstant_Hercules_6.5.4.3_61959")
		if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}
		setup := &fakeSetup{root: dir}
		_, err := Prepare(Request{Interface: "fake", SerialPort: "fake", ServerIP: "169.254.141.5", APBootIP: "169.254.141.6", Netmask: "255.255.255.0", FirmwarePath: path, RootDir: dir, ApprovedSHA256: digest}, setup, nil)
		if err == nil || setup.changes != 0 {
			t.Fatalf("err=%v changes=%d", err, setup.changes)
		}
	}
}
func TestStagingCannotApproveChangedFirmware(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ArubaInstant_Hercules_6.5.4.3_61959")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := firmware.Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	setup := &fakeSetup{root: dir}
	_, err = Prepare(Request{Interface: "fake", SerialPort: "fake", ServerIP: "169.254.141.5", APBootIP: "169.254.141.6", Netmask: "255.255.255.0", FirmwarePath: path, RootDir: dir, ApprovedSHA256: info.SHA256}, setup, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err=%v", err)
	}
	if setup.changes != 2 {
		t.Fatalf("unexpected operations after changed image: %d", setup.changes)
	}
}
