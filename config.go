package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/peterbourgon/diskv"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func intEnvConfig(i *int, name string) {
	if env, err := strconv.Atoi(os.Getenv(name)); err == nil {
		*i = env
	}
}

func megaIntEnvConfig(f *int, name string) {
	if env, err := strconv.ParseFloat(os.Getenv(name), 64); err == nil {
		*f = int(env * 1000000)
	}
}

func urlEnvConfig(u **url.URL, name string) {
	if env, present := os.LookupEnv(name); present {
		if url, err := url.Parse(env); err == nil {
			*u = url
		}
	}
}

func strEnvConfig(s *string, name string) {
	if env := os.Getenv(name); len(env) > 0 {
		*s = env
	}
}

func strSliceEnvConfig(s *[]string, name string) {
	if env := os.Getenv(name); len(env) > 0 {
		*s = strings.Split(env, ",")
	} else {
		*s = make([]string, 0)
	}
}

func boolEnvConfig(b *bool, name string) {
	*b = false
	if env, err := strconv.ParseBool(os.Getenv(name)); err == nil {
		*b = env
	}
}

func hexEnvConfig(b *[]byte, name string) {
	var err error

	if env := os.Getenv(name); len(env) > 0 {
		if *b, err = hex.DecodeString(env); err != nil {
			log.Fatalf("%s expected to be hex-encoded string\n", name)
		}
	}
}

func hexFileConfig(b *[]byte, filepath string) {
	if len(filepath) == 0 {
		return
	}

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("Can't open file %s\n", filepath)
	}

	src, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalln(err)
	}

	src = bytes.TrimSpace(src)

	dst := make([]byte, hex.DecodedLen(len(src)))
	n, err := hex.Decode(dst, src)
	if err != nil {
		log.Fatalf("%s expected to contain hex-encoded string\n", filepath)
	}

	*b = dst[:n]
}

type config struct {
	Bind            string
	ReadTimeout     int
	WaitTimeout     int
	WriteTimeout    int
	DownloadTimeout int
	Concurrency     int
	MaxClients      int
	TTL             int

	MaxSrcDimension  int
	MaxSrcResolution int

	GZipCompression int

	Key  []byte
	Salt []byte

	AllowOrigins []string

	CacheRoot string
	CacheSize int

	ServerURL *url.URL
}

var conf = config{
	Bind:             ":8080",
	ReadTimeout:      10,
	WriteTimeout:     10,
	DownloadTimeout:  5,
	Concurrency:      runtime.NumCPU() * 2,
	TTL:              3600,
	MaxSrcDimension:  8192,
	MaxSrcResolution: 16800000,
	GZipCompression:  5,
}

var farsparkCache *diskv.Diskv

func initCache() {
	if conf.CacheSize > 0 {
		farsparkCache = diskv.New(diskv.Options{
			BasePath:     conf.CacheRoot,
			Transform:    func(s string) []string { return []string{} },
			CacheSizeMax: uint64(conf.CacheSize),
		})
	} else {
		farsparkCache = nil
	}
}

func init() {
	keypath := flag.String("keypath", "", "path of the file with hex-encoded key")
	saltpath := flag.String("saltpath", "", "path of the file with hex-encoded salt")
	flag.Parse()

	if port := os.Getenv("PORT"); len(port) > 0 {
		conf.Bind = fmt.Sprintf(":%s", port)
	}

	strEnvConfig(&conf.Bind, "FARSPARK_BIND")
	intEnvConfig(&conf.ReadTimeout, "FARSPARK_READ_TIMEOUT")
	intEnvConfig(&conf.WriteTimeout, "FARSPARK_WRITE_TIMEOUT")
	intEnvConfig(&conf.DownloadTimeout, "FARSPARK_DOWNLOAD_TIMEOUT")
	intEnvConfig(&conf.Concurrency, "FARSPARK_CONCURRENCY")
	intEnvConfig(&conf.MaxClients, "FARSPARK_MAX_CLIENTS")

	intEnvConfig(&conf.TTL, "FARSPARK_TTL")

	intEnvConfig(&conf.MaxSrcDimension, "FARSPARK_MAX_SRC_DIMENSION")
	megaIntEnvConfig(&conf.MaxSrcResolution, "FARSPARK_MAX_SRC_RESOLUTION")

	intEnvConfig(&conf.GZipCompression, "FARSPARK_GZIP_COMPRESSION")

	hexEnvConfig(&conf.Key, "FARSPARK_KEY")
	hexEnvConfig(&conf.Salt, "FARSPARK_SALT")

	hexFileConfig(&conf.Key, *keypath)
	hexFileConfig(&conf.Salt, *saltpath)

	strSliceEnvConfig(&conf.AllowOrigins, "FARSPARK_ALLOW_ORIGINS")

	strEnvConfig(&conf.CacheRoot, "FARSPARK_CACHE_ROOT")
	intEnvConfig(&conf.CacheSize, "FARSPARK_CACHE_SIZE")

	urlEnvConfig(&conf.ServerURL, "FARSPARK_SERVER_URL")

	if len(conf.Key) == 0 {
		log.Fatalln("Key is not defined")
	}
	if len(conf.Salt) == 0 {
		log.Fatalln("Salt is not defined")
	}

	if len(conf.Bind) == 0 {
		log.Fatalln("Bind address is not defined")
	}

	if conf.ReadTimeout <= 0 {
		log.Fatalf("Read timeout should be greater than 0, now - %d\n", conf.ReadTimeout)
	}

	if conf.WriteTimeout <= 0 {
		log.Fatalf("Write timeout should be greater than 0, now - %d\n", conf.WriteTimeout)
	}

	if conf.DownloadTimeout <= 0 {
		log.Fatalf("Download timeout should be greater than 0, now - %d\n", conf.DownloadTimeout)
	}

	if conf.Concurrency <= 0 {
		log.Fatalf("Concurrency should be greater than 0, now - %d\n", conf.Concurrency)
	}

	if conf.MaxClients <= 0 {
		conf.MaxClients = conf.Concurrency * 10
	}

	if conf.TTL <= 0 {
		log.Fatalf("TTL should be greater than 0, now - %d\n", conf.TTL)
	}

	if conf.MaxSrcDimension <= 0 {
		log.Fatalf("Max src dimension should be greater than 0, now - %d\n", conf.MaxSrcDimension)
	}

	if conf.MaxSrcResolution <= 0 {
		log.Fatalf("Max src resolution should be greater than 0, now - %d\n", conf.MaxSrcResolution)
	}

	if conf.GZipCompression < 0 {
		log.Fatalf("GZip compression should be greater than or quual to 0, now - %d\n", conf.GZipCompression)
	} else if conf.GZipCompression > 9 {
		log.Fatalf("GZip compression can't be greater than 9, now - %d\n", conf.GZipCompression)
	}

	initDownloading()
	initCache()
}
