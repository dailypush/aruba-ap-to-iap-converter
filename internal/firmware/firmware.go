package firmware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Info struct {
	Path    string
	Name    string
	Version string
	SHA256  string
	Size    int64
}

var herculesVersion = regexp.MustCompile(`(?i)ArubaInstant_Hercules_([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`)

func Validate(path string) (Info, error) {
	var info Info
	if path == "" {
		return info, errors.New("firmware image is required")
	}
	st, err := os.Stat(path)
	if err != nil {
		return info, err
	}
	if st.IsDir() {
		return info, errors.New("firmware image path is a directory")
	}

	name := filepath.Base(path)
	lower := strings.ToLower(name)
	if !strings.Contains(lower, "arubainstant") || !strings.Contains(lower, "hercules") {
		return info, errors.New("firmware must be an ArubaInstant Hercules image")
	}
	if strings.Contains(lower, "arubainstant_hercules_8.") {
		return info, errors.New("ArubaOS 8 is rejected for converted 256 MB AP-325 units")
	}
	match := herculesVersion.FindStringSubmatch(name)
	if len(match) != 2 || !strings.HasPrefix(match[1], "6.") {
		return info, errors.New("firmware must be an ArubaOS 6 Instant Hercules image")
	}

	hash, err := sha256File(path)
	if err != nil {
		return info, err
	}

	return Info{
		Path:    path,
		Name:    name,
		Version: match[1],
		SHA256:  hash,
		Size:    st.Size(),
	}, nil
}

func sha256File(path string) (string, error) {
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
