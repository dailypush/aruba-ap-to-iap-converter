package parser

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type MfgInfo struct {
	SystemSerial string
	WiredMAC     string
}

var serialRe = regexp.MustCompile(`(?is)Card\s+0:\s*System.*?Serial\s*:\s*([A-Za-z0-9]+)`)
var wiredMACRe = regexp.MustCompile(`(?i)Wired MAC\s*:\s*([0-9a-f:]{17})`)

func ParseMfgInfo(text string) (MfgInfo, error) {
	var out MfgInfo
	serial := serialRe.FindStringSubmatch(text)
	if len(serial) != 2 {
		return out, fmt.Errorf("Card 0 System serial not found")
	}
	out.SystemSerial = strings.ToUpper(serial[1])

	mac := wiredMACRe.FindStringSubmatch(text)
	if len(mac) == 2 {
		out.WiredMAC = strings.ToLower(mac[1])
	} else {
		out.WiredMAC = "UNKNOWN"
	}
	return out, nil
}

func CountOSInfoVersion(text, version string) int {
	pattern := regexp.MustCompile(`(?i)version:\s*` + regexp.QuoteMeta(version))
	return len(pattern.FindAllString(text, -1))
}

func CountryCodeCommand(country, serial string) string {
	country = strings.ToUpper(country)
	serial = strings.ToUpper(serial)
	sum := sha1.Sum([]byte(country + "-" + serial))
	return "proginv system ccode CCODE-" + country + "-" + hex.EncodeToString(sum[:])
}

func DecodeDefaultIP(hexIP string) (net.IP, error) {
	hexIP = strings.TrimSpace(strings.TrimSuffix(hexIP, ","))
	if len(hexIP) != 8 {
		return nil, fmt.Errorf("default IP token must be 8 hex characters")
	}
	parts := make([]byte, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseUint(hexIP[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		parts[i] = byte(v)
	}
	return net.IPv4(parts[0], parts[1], parts[2], parts[3]), nil
}

func ParseDefaultIP(text string) (net.IP, bool) {
	re := regexp.MustCompile(`(?i)Picked\s+up\s+default\s+IP\s+((?:[0-9a-f]\s*){8})`)
	m := re.FindAllStringSubmatch(text, -1)
	if len(m) == 0 {
		return nil, false
	}
	hexIP := regexp.MustCompile(`(?i)[^0-9a-f]`).ReplaceAllString(m[len(m)-1][1], "")
	ip, err := DecodeDefaultIP(hexIP)
	if err != nil {
		return nil, false
	}
	return ip, true
}

func HasPanic(text string) bool {
	re := regexp.MustCompile(`(?i)Kernel panic|panic:|Invalid image format|rebooting in`)
	return re.MatchString(text)
}
