package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// NewHTTPProxy 创建HTTP代理
func NewHTTPProxy(listen string, rules map[string]*url.URL) *HTTPProxy {
	return &HTTPProxy{
		listen: listen,
		rules:  rules,
		proxy:  new(httputil.ReverseProxy),
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// HTTPProxy HTTP代理
type HTTPProxy struct {
	// The transport used to perform proxy requests.
	// If nil, http.DefaultTransport is used.
	Transport http.RoundTripper

	// FlushInterval specifies the flush interval
	// to flush to the client while copying the
	// response body.
	// If zero, no periodic flushing is done.
	FlushInterval time.Duration

	reqNum int

	proxy *httputil.ReverseProxy

	listen string
	rules  map[string]*url.URL
}

// Run 启动代理
func (p *HTTPProxy) Run() error {
	log.Printf("HTTPProxy listening on %s\n", p.listen)

	if p.Transport == nil {
		p.Transport = http.DefaultTransport
	}

	p.proxy.Director = p.Director
	p.proxy.Transport = p.Transport

	return http.ListenAndServe(p.listen, p.proxy)
}

// Director must be a function which modifies
// the request into a new request to be sent
// using Transport. Its response is then copied
// back to the original client unmodified.
func (p *HTTPProxy) Director(req *http.Request) {
	p.reqNum++

	var uri string
	var pair1, pair2 []string
	var target *url.URL
	var reqURL = new(url.URL)

	reqURL.Host = req.Host
	reqURL.RawPath = req.URL.RawPath
	if nil == req.TLS {
		reqURL.Scheme = "http"
	} else {
		reqURL.Scheme = "https"
	}

	uri = strings.ToLower(reqURL.String())

	for k, v := range p.rules {
		if strings.HasPrefix(uri, k) {
			target = v
			break
		} else if strings.HasPrefix(k, "http://*:") || strings.HasPrefix(k, "http://0.0.0.0:") {
			pair1 = strings.Split(k, ":")
			pair1 = strings.Split(pair1[2], "/")
			pair2 = strings.Split(req.Host, ":")
			pair2 = strings.Split(pair2[1], "/")

			if pair1[0] == pair2[0] {
				target = v
				break
			}
		}
	}
	if nil != target {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if target.RawQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}
}
