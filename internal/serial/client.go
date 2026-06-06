package serial

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"time"

	tarmserial "github.com/tarm/serial"
)

type Client struct {
	port io.ReadWriteCloser
	log  func(string)
}

func Open(path string, baud int, log func(string)) (*Client, error) {
	p, err := tarmserial.OpenPort(&tarmserial.Config{Name: path, Baud: baud, ReadTimeout: 250 * time.Millisecond})
	if err != nil {
		return nil, err
	}
	return &Client{port: p, log: log}, nil
}

func (c *Client) Close() error {
	return c.port.Close()
}

func (c *Client) SendLine(line string) error {
	if c.log != nil {
		c.log(line)
	}
	_, err := c.port.Write([]byte(line + "\r"))
	return err
}

func (c *Client) ReadUntil(pattern *regexp.Regexp, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var buf bytes.Buffer
	tmp := make([]byte, 512)
	for time.Now().Before(deadline) {
		n, err := c.port.Read(tmp)
		if n > 0 {
			chunk := string(tmp[:n])
			buf.Write(tmp[:n])
			if c.log != nil {
				c.log(chunk)
			}
			if pattern.MatchString(buf.String()) {
				return buf.String(), nil
			}
		}
		if err != nil && err != io.EOF {
			return buf.String(), err
		}
	}
	return buf.String(), ErrTimeout
}

func (c *Client) Drain(quietFor time.Duration) string {
	deadline := time.Now().Add(quietFor)
	var buf bytes.Buffer
	tmp := make([]byte, 512)
	for time.Now().Before(deadline) {
		n, err := c.port.Read(tmp)
		if n > 0 {
			chunk := string(tmp[:n])
			buf.Write(tmp[:n])
			if c.log != nil && strings.TrimSpace(chunk) != "" {
				c.log(chunk)
			}
			deadline = time.Now().Add(quietFor)
		}
		if err != nil && err != io.EOF {
			break
		}
	}
	return buf.String()
}

func (c *Client) CatchAPBoot(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var buf bytes.Buffer
	tmp := make([]byte, 512)
	apboot := regexp.MustCompile(`(?s)apboot\s*> ?$`)
	for time.Now().Before(deadline) {
		_, _ = c.port.Write([]byte("\r"))
		n, err := c.port.Read(tmp)
		if n > 0 {
			chunk := string(tmp[:n])
			buf.Write(tmp[:n])
			if c.log != nil {
				c.log(chunk)
			}
			if apboot.MatchString(buf.String()) {
				_ = c.Drain(300 * time.Millisecond)
				return buf.String(), nil
			}
		}
		if err != nil && err != io.EOF {
			return buf.String(), err
		}
	}
	return buf.String(), ErrTimeout
}

func (c *Client) ExpectAPBoot(timeout time.Duration) (string, error) {
	_, _ = c.port.Write([]byte("\r"))
	out, err := c.ReadUntil(regexp.MustCompile(`(?s)apboot\s*> ?$`), timeout)
	if err == nil {
		_ = c.Drain(300 * time.Millisecond)
	}
	return out, err
}

var ErrTimeout = timeoutError{}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "serial timeout"
}
