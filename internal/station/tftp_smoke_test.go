package station

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/platform"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/tftphelper"
)

// Opt-in read-only check of an already configured macOS station. It does not
// configure interfaces, start services, open serial ports, or send AP commands.
func TestNativeTFTPSmoke(t *testing.T) {
	path := os.Getenv("IAP325_TFTP_SMOKE_IMAGE")
	if path == "" || runtime.GOOS != "darwin" {
		t.Skip("set IAP325_TFTP_SMOKE_IMAGE to check the local macOS TFTP service")
	}
	info, err := firmware.Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	address := platform.TFTPVerifyAddress("169.254.141.5")
	if err := tftphelper.Verify(address, "aruba/"+info.Name, info.SHA256, 20*time.Second); err != nil {
		t.Fatal(err)
	}
	t.Logf("Downloaded and verified %d firmware bytes through %s", info.Size, address)
}
