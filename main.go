package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// this program compare transfet speed of the Golang http server (and client) betwen HTTP1.1 and HTTP2
// to avoid encryption ovearhead it uses H2C mode
// usage: run with no argument to do a test with 10G of data
// use the "-s" option to be in server only mode and use another program like curl (or https://nspeed.app) to test

// build a 1MiB buffer of random data
const MaxChunkSize = 1024 * 1024 // warning : 1 MiB //this will be allocated in memory
var BigChunk [MaxChunkSize]byte

func InitBigChunk(seed int64) {
	rng := rand.New(rand.NewSource(seed))
	for i := int64(0); i < MaxChunkSize; i++ {
		BigChunk[i] = byte(rng.Intn(256))
	}
}

// regexp to parse url
var StreamPathRegexp *regexp.Regexp

func createHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	var handler http.Handler = mux
	return handler
}

// handle the only route: '/number' which send <number> bytes of random data
func rootHandler(w http.ResponseWriter, r *http.Request) {

	//fmt.Printf("request from %s: %s\n", r.RemoteAddr, r.URL)
	method := r.Method
	if method == "" {
		method = "GET"
	}
	if method == "GET" {
		match := StreamPathRegexp.FindStringSubmatch(r.URL.Path[1:])
		if len(match) == 2 {
			n, err := strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			streamBytes(w, r, n)
			return
		} else {
			http.Error(w, "Not found (no regexp match)", http.StatusNotFound)
			return
		}
	}
	http.Error(w, "unhandled method", http.StatusBadRequest)
}

// send 'size' bytes of random data
func streamBytes(w http.ResponseWriter, r *http.Request, size int64) {

	// the buffer we use to send data
	var chunkSize int64 = 32 * 1024 // 32K chunk (sweet spot value may depend on OS & hardware)
	if chunkSize > MaxChunkSize {
		log.Fatal("chunksize is too big")
	}
	chunk := BigChunk[:chunkSize]

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	//fmt.Printf("header sent to %s: %s\n", r.RemoteAddr, r.URL)

	hasEnded := false
	var numChunk = size / chunkSize
	for i := int64(0); i < numChunk; i++ {
		_, err := w.Write(chunk)
		if err != nil {
			hasEnded = true
			break
		}
	}
	if size%chunkSize > 0 && !hasEnded {
		w.Write(chunk[:size%chunkSize])
	}

	f := w.(http.Flusher)
	f.Flush()
}

// create a HTTP server
func createServer(ctx context.Context, host string, port int, useH2C bool) *http.Server {
	listenAddr := net.JoinHostPort(host, strconv.Itoa(port))
	server := &http.Server{
		Addr:    listenAddr,
		Handler: createHandler(),
	}
	if useH2C {
		server.Handler = h2c.NewHandler(server.Handler, &http2.Server{})
	}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatalf("cannot listen to %s: %s", server.Addr, err)
	}
	// this spawns the server
	go func() {
		err = server.Serve(ln)
		if err != nil {
			log.Fatalf("cannot serve %s: %s", server.Addr, err)
		}
	}()

	// this will wait for ctx.Done then shutdown the server
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	return server
}

// http client, download the url to 'null' (discard)
func Download(ctx context.Context, url string, useH2C bool) error {

	var dialer = &net.Dialer{
		Timeout:       5 * time.Second, // fail quick
		FallbackDelay: -1,              // don't use Happy Eyeballs
	}
	var netTransport = &http.Transport{
		DialContext: dialer.DialContext,
	}
	var rt http.RoundTripper = netTransport

	if useH2C {
		rt = &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
	}
	c := &http.Client{
		Transport: rt,
	}

	var body io.ReadCloser = http.NoBody

	req, err := http.NewRequestWithContext(ctx, "GET", url, body)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)

	if err == nil && resp != nil {
		fmt.Printf("receiving data with %s\n", resp.Proto)
		startDate := time.Now()
		var totalReceived int64 = 0
		totalReceived, err = io.Copy(ioutil.Discard, resp.Body)
		duration := time.Since(startDate)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return err
			}
		}
		fmt.Printf("received %d bytes in %v = %s\n", totalReceived, duration, FormatBitperSecond(duration.Seconds(), totalReceived))
	}
	return err
}

var optTest = flag.Bool("s", false, "server mode only")

func main() {

	flag.Parse()

	StreamPathRegexp = regexp.MustCompile("^(" + "[0-9]+" + ")$")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// this wait for ctrl-c or kill signal and call cancel()
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	//1. create a http server
	s1 := createServer(ctx, "", 8765, true)
	fmt.Printf("server created and listening at %s (http1.1)\n", s1.Addr)

	//2. create a http/2 (h2c) server
	s2 := createServer(ctx, "", 9876, true)
	fmt.Printf("server created and listening at %s (http/2 cleartext)\n", s2.Addr)

	// if server mode, just wait forever for something else to cancel
	if *optTest {
		fmt.Printf("server mode on\n")
		<-ctx.Done()
	}

	//3. transfert 10G with http server
	s1url := "http://localhost:8765/10000000000"
	fmt.Printf("downloading %s\n", s1url)
	err := Download(ctx, s1url, false)
	if err != nil {
		fmt.Printf("client error for %s: %s\n", s1url, err)
	}

	//4. transfert 10G with http/2 server
	s2url := "http://localhost:9876/10000000000"
	fmt.Printf("downloading %s\n", s2url)
	err = Download(ctx, s2url, true)
	if err != nil {
		fmt.Printf("client error for %s: %s\n", s2url, err)
	}
	cancel()
}

// human friendly formatting stuff

// FormatBitperSecond format bit per seconds in human readable format
func FormatBitperSecond(elapsedSeconds float64, totalBytes int64) string {
	// nyi - fix me
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered from divide by zero")
		}
	}()
	speed := "(too fast)"
	if elapsedSeconds > 0 {
		speed = ByteCountDecimal((int64)((float64)(totalBytes)*8.0/elapsedSeconds)) + "bps"
	}
	return speed
}

// ByteCountDecimal format byte size to human readable format (decimal units)
// suitable to append the unit name after (B, bps, etc)
func ByteCountDecimal(b int64) string {
	s, u := byteCount(b, 1000, "kMGTPE")
	return s + " " + u
}

// copied from : https://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func byteCount(b int64, unit int64, units string) (string, string) {
	if b < unit {
		return fmt.Sprintf("%d", b), ""
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if exp >= len(units) {
		return fmt.Sprintf("%d", b), ""
	}
	return fmt.Sprintf("%.1f", float64(b)/float64(div)), units[exp : exp+1]
}