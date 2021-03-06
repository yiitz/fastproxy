package uri

import (
	"bytes"
	"net"
	"strings"
)

//URI http URI helper
type URI struct {
	isConnect bool

	full   []byte
	scheme []byte
	host   []byte

	path      []byte
	queries   []byte
	fragments []byte

	hostInfo HostInfo

	pathWithQueryFragment       []byte
	pathWithQueryFragmentParsed bool
}

//Scheme ...
func (uri *URI) Scheme() []byte {
	return uri.scheme
}

//Host host specified in uri
func (uri *URI) Host() []byte {
	return uri.host
}

//PathWithQueryFragment ...
func (uri *URI) PathWithQueryFragment() []byte {
	if uri.pathWithQueryFragmentParsed {
		return uri.pathWithQueryFragment
	}
	if uri.isConnect {
		uri.pathWithQueryFragment = nil
		uri.pathWithQueryFragmentParsed = true
		return nil
	}
	if len(uri.host) == 0 {
		uri.pathWithQueryFragment = uri.full
	} else if hostIndex := bytes.Index(uri.full, uri.host); hostIndex >= 0 {
		uri.pathWithQueryFragment = uri.full[hostIndex+len(uri.host):]
	}
	if len(uri.pathWithQueryFragment) == 0 {
		uri.pathWithQueryFragment = uri.path
	}
	uri.pathWithQueryFragmentParsed = true
	return uri.pathWithQueryFragment
}

//Path ...
func (uri *URI) Path() []byte {
	return uri.path
}

//Queries ...
func (uri *URI) Queries() []byte {
	return uri.queries
}

//Fragments ...
func (uri *URI) Fragments() []byte {
	return uri.fragments
}

//HostInfo the host info
func (uri *URI) HostInfo() *HostInfo {
	return &uri.hostInfo
}

//Reset reset the request URI
func (uri *URI) Reset() {
	uri.isConnect = false
	uri.full = uri.full[:0]
	uri.host = uri.host[:0]
	uri.hostInfo.reset()
	uri.scheme = uri.scheme[:0]
	uri.path = uri.path[:0]
	uri.queries = uri.queries[:0]
	uri.fragments = uri.fragments[:0]
	uri.pathWithQueryFragment = uri.pathWithQueryFragment[:0]
	uri.pathWithQueryFragmentParsed = false
}

// ChangeHost change the URI's host
func (uri *URI) ChangeHost(hostWithPort string) {
	if uri.hostInfo.hostWithPort == hostWithPort {
		return
	}
	var newRawURI []byte
	if len(uri.host) == 0 {
		// not host in URI before, add it
		newRawURI = []byte(hostWithPort)
		if len(uri.full) == 0 || (len(uri.full) > 0 && uri.full[0] != '/') {
			newRawURI = append(newRawURI, '/')
		}
		newRawURI = append(newRawURI, uri.full...)
	} else if hostIndex := bytes.Index(uri.full, uri.host); hostIndex >= 0 {
		if len(hostWithPort) == 0 {
			newRawURI = uri.full[hostIndex+len(uri.host):]
		} else {
			// host already in URI, replace it
			newRawURI = bytes.Replace(uri.full, uri.host, []byte(hostWithPort), 1)
		}
	}
	if len(newRawURI) == 0 {
		newRawURI = []byte("/")
	}
	uri.Parse(uri.isConnect, newRawURI)
}

// ChangePathWithFragment change URI's path with fragment
func (uri *URI) ChangePathWithFragment(newPathWithFragment []byte) {
	if uri.isConnect {
		return
	}
	if bytes.Equal(newPathWithFragment, uri.PathWithQueryFragment()) {
		return
	}
	var newRawURI []byte
	if len(uri.host) == 0 {
		newRawURI = newPathWithFragment
	} else if hostIndex := bytes.Index(uri.full, uri.host); hostIndex >= 0 {
		// host already in URI, replace it
		hostEndIndex := hostIndex + len(uri.host)
		newRawURI = uri.full[:hostEndIndex]
		if len(newPathWithFragment) == 0 || (len(newPathWithFragment) > 0 && newPathWithFragment[0] != '/') {
			newRawURI = append(newRawURI, '/')
		}
		newRawURI = append(newRawURI, newPathWithFragment...)
	}
	if len(newRawURI) == 0 {
		newRawURI = []byte("/")
	}
	uri.Parse(uri.isConnect, newRawURI)
}

//Parse parse the request URI
func (uri *URI) Parse(isConnect bool, reqURI []byte) {
	uri.Reset()
	uri.isConnect = isConnect
	uri.full = reqURI
	if len(reqURI) == 0 {
		return
	}
	fragmentIndex := bytes.IndexByte(reqURI, '#')
	if fragmentIndex >= 0 {
		uri.fragments = reqURI[fragmentIndex:]
		uri.parseWithoutFragments(reqURI[:fragmentIndex])
	} else {
		uri.parseWithoutFragments(reqURI)
	}
	if !isConnect && len(uri.path) == 0 {
		uri.path = []byte("/")
	}
	if isConnect {
		uri.scheme = uri.scheme[:0]
		uri.path = uri.path[:0]
		uri.queries = uri.queries[:0]
		uri.fragments = uri.fragments[:0]
	}
	uri.hostInfo.ParseHostWithPort(string(uri.host), isConnect)
}

//parse uri with out fragments
func (uri *URI) parseWithoutFragments(reqURI []byte) {
	if len(reqURI) == 0 {
		return
	}
	queryIndex := bytes.IndexByte(reqURI, '?')
	if queryIndex >= 0 {
		uri.queries = reqURI[queryIndex:]
		uri.parseWithoutQueriesFragments(reqURI[:queryIndex])
	} else {
		uri.parseWithoutQueriesFragments(reqURI)
	}
}

//parse uri without queries and fragments
func (uri *URI) parseWithoutQueriesFragments(reqURI []byte) {
	if len(reqURI) == 0 {
		return
	}
	schemeEnd := getSchemeIndex(reqURI)
	if schemeEnd >= 0 {
		uri.scheme = reqURI[:schemeEnd]
		uri.parseWithoutSchemeQueriesFragments(reqURI[schemeEnd+1:])
	} else {
		uri.parseWithoutSchemeQueriesFragments(reqURI)
	}
}

//parse uri without scheme, queries and fragments
func (uri *URI) parseWithoutSchemeQueriesFragments(reqURI []byte) {
	//remove slashes begin with `//`
	if len(uri.scheme) > 0 && len(reqURI) >= 2 && reqURI[0] == '/' && reqURI[1] == '/' {
		slashIndex := 0
		for i, b := range reqURI {
			if b != '/' {
				break
			}
			slashIndex = i
		}
		reqURI = reqURI[slashIndex+1:]
	}
	if len(reqURI) == 0 {
		return
	}
	//only path
	if reqURI[0] == '/' {
		uri.path = reqURI
		return
	}
	//host with path
	hostNameEnd := bytes.IndexByte(reqURI, '/')
	if hostNameEnd > 0 {
		uri.host = reqURI[:hostNameEnd]
		uri.path = reqURI[hostNameEnd:]
	} else {
		uri.host = reqURI
	}
}

//getSchemeIndex (Scheme must be [a-zA-Z0-9]*)
func getSchemeIndex(rawURL []byte) int {
	for i := 0; i < len(rawURL); i++ {
		c := rawURL[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9':
		case c == ':':
			if i == 0 {
				return 0
			}
			return i
		default:
			// we have encountered an invalid character,
			// so there is no valid scheme
			return -1
		}
	}
	return -1
}

// HostInfo host info
// TODO: test host info
type HostInfo struct {
	domain       string
	ip           net.IP
	port         string
	hostWithPort string
	// ip with port if ip not nil, else domain with port
	targetWithPort string
}

// reset the host info
func (h *HostInfo) reset() {
	h.domain = ""
	h.ip = nil
	h.port = ""
	h.hostWithPort = ""
	h.targetWithPort = ""
}

// Domain return domain
func (h *HostInfo) Domain() string {
	return h.domain
}

// IP return ip
func (h *HostInfo) IP() net.IP {
	return h.ip
}

// Port return port
func (h *HostInfo) Port() string {
	return h.port
}

// HostWithPort return hostWithPort
func (h *HostInfo) HostWithPort() string {
	return h.hostWithPort
}

// TargetWithPort return targetWithPort
func (h *HostInfo) TargetWithPort() string {
	return h.targetWithPort
}

// ParseHostWithPort parse host with port, and set host, ip,
// port, hostWithPort, targetWithPort
func (h *HostInfo) ParseHostWithPort(host string, isHTTPS bool) {
	hasPortFuncByte := func(host string) bool {
		return strings.LastIndexByte(host, ':') >
			strings.LastIndexByte(host, ']')
	}
	if len(host) == 0 {
		return
	}

	// separate domain and port
	if !hasPortFuncByte(host) {
		h.domain = host
		if isHTTPS {
			h.port = "443"
		} else {
			h.port = "80"
		}
	} else {
		var err error
		h.domain, h.port, err = net.SplitHostPort(host)
		if err != nil {
			h.reset()
			return
		}
	}
	if len(h.domain) == 0 {
		return
	}

	// determine whether the given domain is already an IP Address
	ip := net.ParseIP(h.domain)
	if ip != nil {
		h.ip = ip
	}

	// host and target with port
	h.hostWithPort = h.domain + ":" + h.port
	h.targetWithPort = h.hostWithPort
}

// SetIP set ip and update targetWithPort
func (h *HostInfo) SetIP(ip net.IP) {
	if ip == nil {
		return
	}
	h.ip = ip
	h.targetWithPort = ip.String() + ":" + h.port
}
