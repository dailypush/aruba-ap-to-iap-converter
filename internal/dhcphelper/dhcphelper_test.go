package dhcphelper

import (
	"encoding/binary"
	"net"
	"testing"
)

func requestPacket(options []byte) []byte {
	packet := make([]byte, 240)
	packet[0], packet[1], packet[2] = 1, 1, 6
	copy(packet[28:34], []byte{0x40, 0xe3, 0xd6, 0xce, 0x19, 0x9e})
	binary.BigEndian.PutUint32(packet[236:240], 0x63825363)
	return append(packet, options...)
}
func TestDHCPRequestSelection(t *testing.T) {
	cfg := Config{ServerIP: net.ParseIP("169.254.141.5"), LeaseIP: net.ParseIP("169.254.141.6")}
	for _, tc := range []struct {
		name    string
		options []byte
		mac     string
		want    bool
	}{
		{"discover", []byte{53, 1, 1, 255}, "", true},
		{"selected server", []byte{53, 1, 3, 54, 4, 169, 254, 141, 5, 50, 4, 169, 254, 141, 6, 255}, "", true},
		{"other server", []byte{53, 1, 3, 54, 4, 169, 254, 141, 9, 255}, "", false},
		{"other address", []byte{53, 1, 3, 50, 4, 169, 254, 141, 9, 255}, "", false},
		{"second device", []byte{53, 1, 1, 255}, "40:e3:d6:ce:19:ff", false},
		{"same device", []byte{53, 1, 1, 255}, "40:e3:d6:ce:19:9e", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := parse(requestPacket(tc.options))
			if err != nil {
				t.Fatal(err)
			}
			if got := acceptRequest(msg, cfg, tc.mac); got != tc.want {
				t.Fatalf("accepted=%v want=%v", got, tc.want)
			}
		})
	}
}
func TestRejectMalformedDHCP(t *testing.T) {
	for _, opts := range [][]byte{{53}, {53, 2, 1}, {53, 2, 1, 1}, {54, 3, 1, 2, 3}} {
		if _, err := parse(requestPacket(opts)); err == nil {
			t.Fatalf("accepted malformed options %v", opts)
		}
	}
	packet := requestPacket([]byte{53, 1, 1, 255})
	packet[24] = 1
	if _, err := parse(packet); err == nil {
		t.Fatal("accepted relayed request")
	}
}
