package webprobe

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func Probe(ctx context.Context, ip, iface string) (string, bool) {
	candidates := []string{
		"https://" + ip + "/",
		"http://" + ip + "/",
		"https://" + ip + ":4343/",
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).DialContext,
	}
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	for _, url := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return url, true
		}
	}
	return "", false
}
