package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
var serialPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// Validate checks operator inputs before any station or AP changes.
func (c Config) Validate() error {
	if strings.TrimSpace(c.FirmwarePath) == "" {
		return fmt.Errorf("select a firmware image")
	}
	if strings.TrimSpace(c.SerialPort) == "" {
		return fmt.Errorf("serial port is required")
	}
	if strings.TrimSpace(c.Interface) == "" {
		return fmt.Errorf("Ethernet interface is required")
	}
	if err := ValidateCountry(c.Country); err != nil {
		return err
	}
	return ValidateNetwork(c.ServerIP, c.APBootIP, c.APNetmask)
}

func ValidateCountry(country string) error {
	if !countryPattern.MatchString(country) {
		return fmt.Errorf("country must be a two-letter uppercase code, for example US")
	}
	return nil
}

func ValidateSerial(serial string) error {
	if serial != "" && !serialPattern.MatchString(serial) {
		return fmt.Errorf("expected serial must contain only letters and digits")
	}
	return nil
}

func ValidateNetwork(server, ap, mask string) error {
	serverIP, apIP, maskIP := net.ParseIP(server).To4(), net.ParseIP(ap).To4(), net.ParseIP(mask).To4()
	if serverIP == nil || apIP == nil || maskIP == nil {
		return fmt.Errorf("server IP, AP IP, and netmask must be IPv4 addresses")
	}
	subnetMask := net.IPMask(maskIP)
	ones, bits := subnetMask.Size()
	if bits != 32 || ones < 1 || ones > 30 {
		return fmt.Errorf("netmask must be contiguous and allow two hosts (/1 through /30)")
	}
	if serverIP.Equal(apIP) {
		return fmt.Errorf("server and AP must have different IP addresses")
	}
	network := &net.IPNet{IP: serverIP.Mask(subnetMask), Mask: subnetMask}
	if !network.Contains(apIP) {
		return fmt.Errorf("server and AP must be in the same subnet")
	}
	for _, ip := range []net.IP{serverIP, apIP} {
		if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip[0] == 0 || ip[0] >= 224 {
			return fmt.Errorf("%s is not a usable station address", ip)
		}
		host, broadcast := false, true
		for i := 0; i < 4; i++ {
			host = host || ip[i]&^subnetMask[i] != 0
			broadcast = broadcast && ip[i]|subnetMask[i] == 255
		}
		if !host || broadcast {
			return fmt.Errorf("%s is a network or broadcast address", ip)
		}
	}
	return nil
}
