package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func main() {
	targetRawURL := os.Getenv("PROXY_TARGET_URL")
	if targetRawURL == "" {
		log.Fatal(`"PROXY_TARGET_URL" environment variable MUST be set`)
	}

	rURL, err := url.Parse(targetRawURL)
	fatalOnError(err)

	proxy := httputil.NewSingleHostReverseProxy(rURL)
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	http.Handle("/", proxy)

	err = http.ListenAndServe(":8080", nil)
	fatalOnError(err)
}

func fatalOnError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
