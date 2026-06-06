package tftphelper

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	opRRQ   = 1
	opDATA  = 3
	opACK   = 4
	opERROR = 5
)

type Config struct {
	Root     string
	Address  string
	Duration time.Duration
	Log      func(string)
}

func Run(cfg Config) error {
	if cfg.Root == "" {
		return errors.New("TFTP root is required")
	}
	if cfg.Address == "" {
		cfg.Address = ":69"
	}
	if cfg.Duration == 0 {
		cfg.Duration = 30 * time.Minute
	}
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	log := func(s string) {
		if cfg.Log != nil {
			cfg.Log(s)
		}
	}
	addr, err := net.ResolveUDPAddr("udp4", cfg.Address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("bind TFTP UDP%s: %w", cfg.Address, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(cfg.Duration)
	log(fmt.Sprintf("TFTP helper listening on %s rooted at %s for %s", cfg.Address, root, cfg.Duration))
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1500)
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if n < 2 {
			continue
		}
		op := binary.BigEndian.Uint16(buf[:2])
		if op != opRRQ {
			_ = sendError(conn, remote, 4, "only RRQ is supported")
			continue
		}
		name, mode, err := parseRRQ(buf[:n])
		if err != nil {
			_ = sendError(conn, remote, 0, err.Error())
			continue
		}
		log(fmt.Sprintf("RRQ %s mode=%s from %s", name, mode, remote))
		go serveFile(root, name, remote, log)
	}
	log("TFTP helper exiting after " + cfg.Duration.String())
	return nil
}

func serveFile(root, name string, remote *net.UDPAddr, log func(string)) {
	path, err := safePath(root, name)
	if err != nil {
		_ = sendErrorTo(remote, 2, err.Error())
		return
	}
	f, err := os.Open(path)
	if err != nil {
		_ = sendErrorTo(remote, 1, "file not found")
		log("TFTP open failed for " + path + ": " + err.Error())
		return
	}
	defer f.Close()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		log("TFTP data socket failed: " + err.Error())
		return
	}
	defer conn.Close()

	block := uint16(1)
	buf := make([]byte, 512)
	for {
		n, readErr := io.ReadFull(f, buf)
		if readErr == io.ErrUnexpectedEOF || readErr == io.EOF {
			readErr = nil
		}
		if readErr != nil {
			_ = sendError(conn, remote, 0, readErr.Error())
			return
		}
		pkt := make([]byte, 4+n)
		binary.BigEndian.PutUint16(pkt[0:2], opDATA)
		binary.BigEndian.PutUint16(pkt[2:4], block)
		copy(pkt[4:], buf[:n])
		if err := sendDataAndWaitACK(conn, remote, pkt, block); err != nil {
			log(fmt.Sprintf("TFTP transfer %s block %d failed: %s", name, block, err))
			return
		}
		if n < 512 {
			log("TFTP transfer complete: " + name)
			return
		}
		block++
	}
}

func sendDataAndWaitACK(conn *net.UDPConn, remote *net.UDPAddr, pkt []byte, block uint16) error {
	ack := make([]byte, 516)
	for attempt := 0; attempt < 8; attempt++ {
		if _, err := conn.WriteToUDP(pkt, remote); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, from, err := conn.ReadFromUDP(ack)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if from.IP.Equal(remote.IP) && from.Port == remote.Port && n >= 4 &&
			binary.BigEndian.Uint16(ack[0:2]) == opACK &&
			binary.BigEndian.Uint16(ack[2:4]) == block {
			return nil
		}
	}
	return fmt.Errorf("timeout waiting for ACK %d", block)
}

func parseRRQ(pkt []byte) (string, string, error) {
	if len(pkt) < 4 {
		return "", "", errors.New("short RRQ")
	}
	parts := strings.Split(string(pkt[2:]), "\x00")
	if len(parts) < 2 || parts[0] == "" {
		return "", "", errors.New("missing RRQ filename")
	}
	mode := strings.ToLower(parts[1])
	if mode == "" {
		mode = "octet"
	}
	return parts[0], mode, nil
}

func safePath(root, name string) (string, error) {
	clean := filepath.Clean(strings.TrimLeft(strings.ReplaceAll(name, "\\", "/"), "/"))
	path := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("invalid TFTP path")
	}
	return path, nil
}

func sendErrorTo(remote *net.UDPAddr, code uint16, msg string) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	defer conn.Close()
	return sendError(conn, remote, code, msg)
}

func sendError(conn *net.UDPConn, remote *net.UDPAddr, code uint16, msg string) error {
	pkt := make([]byte, 4, 4+len(msg)+1)
	binary.BigEndian.PutUint16(pkt[0:2], opERROR)
	binary.BigEndian.PutUint16(pkt[2:4], code)
	pkt = append(pkt, []byte(msg)...)
	pkt = append(pkt, 0)
	_, err := conn.WriteToUDP(pkt, remote)
	return err
}
