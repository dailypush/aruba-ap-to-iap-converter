package inventory

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	Timestamp    time.Time
	Serial       string
	WiredMAC     string
	Country      string
	ImageVersion string
	Status       string
	Stage        string
	Message      string
	LogPath      string
}

type Store struct {
	LogDir string
}

func New(logDir string) Store {
	return Store{LogDir: logDir}
}

func (s Store) AppendResult(r Result) error {
	if err := os.MkdirAll(s.LogDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.LogDir, "conversion-results.csv")
	newFile := !exists(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if newFile {
		if err := w.Write([]string{"timestamp", "serial", "country", "image_version", "status", "stage", "message", "log_path"}); err != nil {
			return err
		}
	}
	if err := w.Write([]string{
		formatTime(r.Timestamp), r.Serial, r.Country, r.ImageVersion, r.Status, r.Stage, r.Message, r.LogPath,
	}); err != nil {
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	if r.Status == "SUCCESS" {
		return s.UpsertInventory(r)
	}
	return nil
}

func (s Store) UpsertInventory(r Result) error {
	if err := os.MkdirAll(s.LogDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(s.LogDir, "ap-inventory.csv")
	header := []string{"converted_at", "serial", "wired_mac", "country", "image_version", "status", "conversion_log"}
	rows := [][]string{header}
	replaced := false

	if exists(path) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		readRows, err := csv.NewReader(f).ReadAll()
		f.Close()
		if err != nil {
			return err
		}
		for i, row := range readRows {
			if i == 0 || len(row) < 2 {
				continue
			}
			if strings.EqualFold(row[1], r.Serial) {
				rows = append(rows, inventoryRow(r))
				replaced = true
			} else {
				rows = append(rows, row)
			}
		}
	}
	if !replaced {
		rows = append(rows, inventoryRow(r))
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		f.Close()
		return err
	}
	w.Flush()
	err = w.Error()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func inventoryRow(r Result) []string {
	return []string{
		formatTime(r.Timestamp), r.Serial, r.WiredMAC, r.Country, r.ImageVersion, r.Status, r.LogPath,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("2006-01-02T15:04:05-0700")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
