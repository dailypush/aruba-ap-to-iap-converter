package platform

import "testing"

func TestLocalTFTPAddress(t *testing.T) {
	for _, tc := range []struct{ goos, want string }{
		{"darwin", "127.0.0.1:69"},
		{"windows", "169.254.141.5:69"},
		{"linux", "169.254.141.5:69"},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			if got := tftpVerifyAddress(tc.goos, "169.254.141.5"); got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}
