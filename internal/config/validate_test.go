package config

import "testing"

func TestValidateNetwork(t *testing.T) {
	for _, tc := range []struct {
		name, server, ap, mask string
		valid                  bool
	}{
		{"direct", "169.254.141.5", "169.254.141.6", "255.255.255.0", true},
		{"same address", "169.254.141.5", "169.254.141.5", "255.255.255.0", false},
		{"different subnet", "192.168.1.2", "192.168.2.3", "255.255.255.0", false},
		{"noncontiguous mask", "192.168.1.2", "192.168.1.3", "255.0.255.0", false},
		{"broadcast", "192.168.1.2", "192.168.1.255", "255.255.255.0", false},
		{"network address", "192.168.1.0", "192.168.1.3", "255.255.255.0", false},
		{"ipv6", "::1", "192.168.1.3", "255.255.255.0", false},
		{"command injection", "192.168.1.2\rboot", "192.168.1.3", "255.255.255.0", false},
		{"multicast", "224.1.1.2", "224.1.1.3", "255.255.255.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateNetwork(tc.server, tc.ap, tc.mask); (err == nil) != tc.valid {
				t.Fatalf("valid=%t: %v", tc.valid, err)
			}
		})
	}
}
