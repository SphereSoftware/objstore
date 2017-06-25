package api

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/astranet/astranet"
	"github.com/astranet/astranet/addr"
	"github.com/gin-gonic/gin"

	"github.com/xlab/objstore"
)

type PrivateServer struct {
	router astranet.AstraNet
	mux    http.Handler

	nodeID string
	debug  bool
	tags   []string
}

func NewPrivateServer(nodeID string, tags ...string) *PrivateServer {
	return &PrivateServer{
		nodeID: nodeID,
		tags:   tags,

		// initializes server+client+router for private net
		router: astranet.New().Router().WithEnv(tags...),
	}
}

func (p *PrivateServer) SetDebug(enabled bool) {
	p.debug = true
}

func (p *PrivateServer) Env() []string {
	return p.tags
}

func (p *PrivateServer) Router() astranet.AstraNet {
	return p.router
}

// ListenAndServe initializes a HTTP listener for private services, starts
// listening on a TCP address for virtual network transport.
func (p *PrivateServer) ListenAndServe(addr string) error {
	listener, err := p.router.Bind("", "objstore-"+p.nodeID)
	if err != nil {
		return err
	}
	if p.debug {
		log.Println("ListenAndServe on", addr, "with service", "objstore-"+p.nodeID)
		log.Println(p.router.Services())
	}
	// start a HTTP server using node's private listener
	go http.Serve(listener, p.mux)

	if err = p.router.ListenAndServe("tcp4", addr); err == nil {
		p.router.Join("tcp4", addr)
	}
	return err
}

const defaultPort = "11999"

// JoinCluster connects to another machines via TCP to join the virtual network.
func (p *PrivateServer) JoinCluster(nodes []string) error {
	var failed []string
	for _, nodeAddr := range nodes {
		if _, _, err := net.SplitHostPort(nodeAddr); err != nil {
			nodeAddr = nodeAddr + ":" + defaultPort
		}
		if err := p.router.Join("tcp4", nodeAddr); err != nil {
			failed = append(failed, nodeAddr)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to join nodes: %v", failed)
	}
	p.router.Services()
	return nil
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

// ExposeAPI initiates HTTP routing to the private API via loopback.
func (p *PrivateServer) ExposeAPI(addr string) error {
	privateProxy := &httputil.ReverseProxy{
		Transport:     newHTTPTransport(p.router),
		FlushInterval: time.Millisecond * 10,
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "objstore-" + p.nodeID
		},
	}
	return http.ListenAndServe(addr, privateProxy)
}

type NodeInfo struct {
	ID    string `json:"id"`
	Addr  string `json:"addr"`
	VAddr string `json:"vaddr"`
}

type NodeIter func(id, addr, vaddr string) error

var (
	RangeStop   = errors.New("stop")
	ForEachStop = RangeStop
)

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

func (p *PrivateServer) RouteAPI(store objstore.Store) {
	r := gin.Default()
	r.GET("/private/v1/ping", p.PingHandler())
	r.GET("/private/v1/nodes", p.ListNodesHandler())
	r.POST("/private/v1/announce", p.AnnounceHandler(store))
	r.GET("/private/v1/get/:id", p.GetHandler(store))
	r.POST("/private/v1/message", p.MessageHandler(store))
	p.mux = r
}

func (p *PrivateServer) PingHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(200, p.nodeID)
	}
}

func (p *PrivateServer) ListNodesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodes []NodeInfo
		if err := forEachNode(p.router, func(id, addr, vaddr string) error {
			nodes = append(nodes, NodeInfo{
				ID:    id,
				Addr:  addr,
				VAddr: vaddr,
			})
			return nil
		}); err != nil {
			c.String(500, "error: %v", err)
			return
		}
		c.JSON(200, nodes)
	}
}

func (p *PrivateServer) AnnounceHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var event *objstore.EventAnnounce
		if err := c.BindJSON(&event); err != nil {
			return
		}
		store.ReceiveEventAnnounce(event)
		c.Status(200)
	}
}

func (p *PrivateServer) MessageHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		r := io.LimitReader(c.Request.Body, 8*1024) // 8kB limit
		data, _ := ioutil.ReadAll(r)
		c.Request.Body.Close()
		store.EmitEventAnnounce(&objstore.EventAnnounce{
			Type:       objstore.EventOpaqueData,
			OpaqueData: data,
		})
		c.Status(200)
	}
}

func (p *PrivateServer) GetHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, meta, err := store.GetObject(c.Param("id"))
		if err == objstore.ErrNotFound {
			c.Status(404)
			return
		} else if err != nil {
			c.String(500, "error: %v", err)
			return
		}
		serveObject(c, r, meta)
	}
}

func serveObject(c *gin.Context, r io.ReadCloser, meta *objstore.FileMeta) {
	ts := time.Unix(meta.Timestamp, 0)
	if seekable, ok := r.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, meta.Name, ts, seekable)
		return
	}
	// actually do all the work http.ServeContent does, but without support
	// of ranges and partial reads due to lack of io.Seeker interface.
	if !ts.IsZero() {
		c.Header("Last-Modified", ts.UTC().Format(http.TimeFormat))
	}
	ctype := mime.TypeByExtension(filepath.Ext(meta.Name))
	c.Header("Content-Type", ctype)
	c.Header("Content-Length", strconv.FormatInt(meta.Size, 10))
	io.CopyN(c.Writer, r, meta.Size)
}
