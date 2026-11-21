package tftphelper

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyDownloadsExactFirmware(t *testing.T) {
	for _, tc := range []struct {
		name      string
		size      int
		wrongHash bool
	}{
		{"short", 17, false}, {"exact block", 512, false}, {"multiple blocks", 1700, false}, {"wrong checksum", 700, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			data := []byte(strings.Repeat("x", tc.size))
			if err := os.WriteFile(filepath.Join(root, "image"), data, 0644); err != nil {
				t.Fatal(err)
			}
			listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = listener.SetReadDeadline(time.Now().Add(5 * time.Second))
				buf := make([]byte, 1500)
				n, remote, err := listener.ReadFromUDP(buf)
				if err != nil {
					return
				}
				name, _, err := parseRRQ(buf[:n])
				if err != nil {
					return
				}
				serveFile(root, name, remote, func(string) {})
			}()
			sum := sha256.Sum256(data)
			digest := hex.EncodeToString(sum[:])
			if tc.wrongHash {
				digest = strings.Repeat("0", 64)
			}
			err = Verify(listener.LocalAddr().String(), "image", digest, 3*time.Second)
			if (err != nil) != tc.wrongHash {
				t.Fatalf("verify err=%v", err)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("server did not finish")
			}
		})
	}
}
func TestRejectSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "image")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := safePath(root, "image"); err == nil {
		t.Fatal("accepted symlink outside TFTP root")
	}
}
func TestRejectMalformedRRQ(t *testing.T) {
	for _, payload := range []string{"image\x00octet", "image\x00netascii\x00", "image\x00\x00"} {
		if _, _, err := parseRRQ(append([]byte{0, opRRQ}, payload...)); err == nil {
			t.Fatalf("accepted %q", payload)
		}
	}
}

func TestACKWaitIgnoresDuplicatePackets(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	done := make(chan error, 1)
	go func() {
		_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 516)
		_, peer, err := client.ReadFromUDP(buf)
		if err != nil {
			done <- err
			return
		}
		for i := 0; i < 12; i++ {
			if _, err = client.WriteToUDP([]byte{0, opACK, 0, 0}, peer); err != nil {
				done <- err
				return
			}
		}
		_, err = client.WriteToUDP([]byte{0, opACK, 0, 1}, peer)
		done <- err
	}()
	if err := sendDataAndWaitACK(server, client.LocalAddr().(*net.UDPAddr), []byte{0, opDATA, 0, 1, 42}, 1); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
