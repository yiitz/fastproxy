package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/haxii/fastproxy/bufiopool"
	"github.com/haxii/fastproxy/client"
	"github.com/haxii/fastproxy/hijack"
	"github.com/haxii/fastproxy/http"
	"github.com/haxii/fastproxy/superproxy"
	"github.com/haxii/log"
)

func TestProxyServe(t *testing.T) {
	go func() {
		ln, err := net.Listen("tcp4", "0.0.0.0:5050")
		if err != nil {
			return
		}
		superProxy, _ := superproxy.NewSuperProxy("0.0.0.0", 8081, superproxy.ProxyTypeSOCKS5, "", "", false)
		proxy := Proxy{
			BufioPool:   &bufiopool.Pool{},
			Client:      client.Client{},
			ProxyLogger: &log.DefaultLogger{},
			Handler: Handler{
				ShouldAllowConnection: func(conn net.Addr) bool {
					fmt.Printf("allowed connection from %s\n", conn.String())
					return true
				},
				ShouldDecryptHost: func(hostWithPort string) bool {
					return true
				},
				URLProxy: func(hostWithPort string, uri []byte) *superproxy.SuperProxy {
					if strings.Contains(hostWithPort, "lumtest") {
						return nil
					}
					if len(uri) == 0 {
						//this is a connections should not decrypt
						fmt.Println(hostWithPort)
					}
					return superProxy
				},
				HijackerPool: &SimpleHijackerPool{},
			},
		}
		if err := proxy.Serve(ln, 30*time.Second); err != nil {
			panic(err)
		}
	}()

	conn, err := net.Dial("tcp", "0.0.0.0:5050")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if status != "HTTP/1.1 400 Bad Request\r\n" {
		t.Fatalf("an error occurred when send get request")
	}
}

//SimpleHijackerPool implements the HijackerPool based on simpleHijacker & sync.Pool
type SimpleHijackerPool struct {
	pool sync.Pool
}

//Get get a simple hijacker from pool
func (p *SimpleHijackerPool) Get(clientAddr net.Addr,
	targetHost string, method, path []byte) hijack.Hijacker {
	v := p.pool.Get()
	var h *simpleHijacker
	if v == nil {
		h = &simpleHijacker{}
	} else {
		h = v.(*simpleHijacker)
	}
	h.Set(clientAddr, targetHost, method, path)
	return h
}

//Put puts a simple hijacker back to pool
func (p *SimpleHijackerPool) Put(s hijack.Hijacker) {
	p.pool.Put(s)
}

type simpleHijacker struct {
	clientAddr, targetHost string
	method, path           []byte
}

func (s *simpleHijacker) Set(clientAddr net.Addr,
	host string, method, path []byte) {
	s.clientAddr = clientAddr.String()
	s.targetHost = host
	s.method = method
	s.path = path
}

func (s *simpleHijacker) OnRequest(header http.Header, rawHeader []byte) io.Writer {
	return nil
}

func (s *simpleHijacker) HijackResponse() io.Reader {
	return nil
}

func (s *simpleHijacker) OnResponse(respLine http.ResponseLine,
	header http.Header, rawHeader []byte) io.Writer {
	return nil
}
