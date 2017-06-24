package cluster

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/astranet/astranet"
	"github.com/astranet/astranet/addr"
)

type PrivateClient struct {
	router astranet.AstraNet
	cli    *http.Client
}

// NewPrivateClient initializes a new client for the virtual network.
// Obtain router handle from an initialized private API server.
func NewPrivateClient(router astranet.AstraNet) *PrivateClient {
	return &PrivateClient{
		router: router,
		cli: &http.Client{
			// TODO: study timeouts for private APIs
			// Timeout:  time.Minute,
			Transport: newHTTPTransport(router),
		},
	}
}

func newHTTPTransport(router astranet.AstraNet) *http.Transport {
	return &http.Transport{
		DisableKeepAlives: true,
		Dial: func(network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			return router.Dial(network, host)
		},
	}
}

func nodeURI(id string) string {
	return "http://objstore-" + id
}

type NodeIter func(id, addr, vaddr string) error

func (p *PrivateClient) ForEachNode(iterFunc NodeIter) error {
	return forEachNode(p.router, iterFunc)
}

func forEachNode(router astranet.AstraNet, iterFunc NodeIter) error {
	services := router.Services()
	seen := make(map[string]bool)
	for _, info := range services {
		if !strings.HasPrefix(info.Service, "objstore-") {
			continue
		}
		if info.Upstream == nil {
			continue
		}
		nodeID := strings.TrimPrefix(strings.Split(info.Service, ".")[0], "objstore-")
		host, _, _ := net.SplitHostPort(info.Upstream.RAddr().String())
		if seen[nodeID+host] {
			continue
		} else {
			seen[nodeID+host] = true
		}
		vaddr := getAddr(info.Host, info.Port)
		if err := iterFunc(nodeID, host, vaddr); err == RangeStop {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

func getAddr(host uint64, port uint32) string {
	return fmt.Sprintf("%s:%d", addr.Uint2Host(host), port)
}

var (
	RangeStop   = errors.New("stop")
	ForEachStop = RangeStop
)

func (p *PrivateClient) GET(nodeID, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("GET", nodeURI(nodeID)+path, nil)
	if err != nil {
		return nil, err
	}
	return p.cli.Do(req)
}

func (p *PrivateClient) POST(nodeID, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", nodeURI(nodeID)+path, body)
	if err != nil {
		return nil, err
	}
	return p.cli.Do(req)
}
