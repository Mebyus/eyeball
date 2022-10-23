package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	timeout       = 10 * time.Second
	maxHeaderSize = 1 << 20

	infoPrefix    = "[info]    "
	errorPrefix   = "[error]   "
	requestPrefix = "[request] "
)

type Counter struct {
	next int
	mu   sync.Mutex
}

func (c *Counter) Next() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.next
	c.next++
	return next
}

type Dumper struct {
	logger      *log.Logger
	loggerError *log.Logger
	counter     Counter

	prefix string
	dir    string
}

func (d *Dumper) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	requestNumber := d.counter.Next()
	d.logger.Printf("#%04d  %-8s %s", requestNumber, req.Method, req.URL.Path)
	filename := filepath.Join(d.dir, fmt.Sprintf("%s%04d.http", d.prefix, requestNumber))
	file, err := os.Create(filename)
	if err != nil {
		d.loggerError.Printf("#%04d  create dump file: %s", requestNumber, err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = req.Write(file)
	if err != nil {
		d.loggerError.Printf("#%04d  write dump: %s", requestNumber, err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func main() {
	loggerInfo := log.New(os.Stdout, infoPrefix, log.Lmsgprefix|log.Ltime)
	loggerError := log.New(os.Stdout, errorPrefix, log.Lmsgprefix|log.Ltime)

	var host, dir, prefix string
	var port uint

	flag.StringVar(&host, "host", "localhost", "HTTP server's host")
	flag.UintVar(&port, "port", 80, "HTTP server's port")
	flag.StringVar(&dir, "dir", ".", "HTTP requests dump directory")
	flag.StringVar(&prefix, "prefix", "request_", "name prefix of request dumps")

	flag.Parse()

	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		loggerError.Fatalln(err)
	}
	absDirPath, err := filepath.Abs(dir)
	if err != nil {
		loggerError.Fatalln(err)
	}

	hostAndPort := net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10))

	server := http.Server{
		Addr: hostAndPort,
		Handler: &Dumper{
			logger:      log.New(os.Stdout, requestPrefix, log.Lmsgprefix|log.Ltime),
			loggerError: loggerError,
			prefix:      prefix,
			dir:         absDirPath,
		},
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: maxHeaderSize,
	}
	loggerInfo.Printf("Dumps dir: %s", absDirPath)
	loggerInfo.Printf("Listening: %s", hostAndPort)
	loggerError.Fatalln(server.ListenAndServe())
}
