package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/xlab/objstore"
)

type PublicServer struct {
	nodeID string

	mux *gin.Engine
}

func NewPublicServer(nodeID string) *PublicServer {
	return &PublicServer{
		nodeID: nodeID,
	}
}

func (p *PublicServer) ListenAndServe(addr string) error {
	return p.mux.Run(addr)
}

func (p *PublicServer) RouteAPI(store objstore.Store) {
	r := gin.Default()
	r.GET("/api/v1/get/:id", p.GetHandler(store))
	r.GET("/api/v1/meta/:id", p.MetaHandler(store))
	r.POST("/api/v1/put", p.PutHandler(store))
	r.POST("/api/v1/delete/:id", p.DeleteHandler(store))
	r.GET("/api/v1/id", p.IDHandler())
	r.GET("/api/v1/version", p.VersionHandler())
	r.GET("/api/v1/ping", p.PingHandler())
	r.GET("/api/v1/stats", p.StatsHandler(store))
	p.mux = r
}

func (p *PublicServer) PingHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(200, p.nodeID)
	}
}

func (p *PublicServer) IDHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(200, objstore.GenerateID())
	}
}

func (p *PublicServer) VersionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: version generation from commit ID
		c.String(200, "dev")
	}
}

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

type DiskStats struct {
	*objstore.DiskStats

	KBytesAll  float64 `json:"kb_all"`
	KBytesUsed float64 `json:"kb_used"`
	KBytesFree float64 `json:"kb_free"`

	MBytesAll  float64 `json:"mb_all"`
	MBytesUsed float64 `json:"mb_used"`
	MBytesFree float64 `json:"mb_free"`

	GBytesAll  float64 `json:"gb_all"`
	GBytesUsed float64 `json:"gb_used"`
	GBytesFree float64 `json:"gb_free"`
}

type Stats struct {
	DiskStats *DiskStats `json:"disk_stats"`
	// TODO: other stats
}

func (p *PublicServer) StatsHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stats Stats
		if ds, err := store.DiskStats(); err == nil {
			stats.DiskStats = &DiskStats{
				DiskStats: ds,
			}
			stats.DiskStats.KBytesAll = float64(ds.BytesAll) / KB
			stats.DiskStats.KBytesUsed = float64(ds.BytesUsed) / KB
			stats.DiskStats.KBytesFree = float64(ds.BytesFree) / KB
			stats.DiskStats.MBytesAll = float64(ds.BytesAll) / MB
			stats.DiskStats.MBytesUsed = float64(ds.BytesUsed) / MB
			stats.DiskStats.MBytesFree = float64(ds.BytesFree) / MB
			stats.DiskStats.GBytesAll = float64(ds.BytesAll) / GB
			stats.DiskStats.GBytesUsed = float64(ds.BytesUsed) / GB
			stats.DiskStats.GBytesFree = float64(ds.BytesFree) / GB
		}
		c.JSON(200, stats)
	}
}

func (p *PublicServer) GetHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var fetch bool
		fetchOption := c.Request.Header.Get("X-Meta-Fetch")
		if strings.ToLower(fetchOption) == "true" || fetchOption == "1" {
			fetch = true
		}
		r, meta, err := store.FindObject(c, c.Param("id"), fetch)
		if err == objstore.ErrNotFound {
			if meta != nil {
				serveMeta(c, meta)
			}
			c.Status(404)
			return
		} else if err != nil {
			c.String(500, "error: %v", err)
			return
		}
		serveObject(c, r, meta)
	}
}

func (p *PublicServer) MetaHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		meta, err := store.HeadObject(c.Param("id"))
		if err == objstore.ErrNotFound {
			if meta != nil {
				serveMeta(c, meta)
			}
			c.Status(404)
			return
		} else if err != nil {
			c.String(500, "error: %v", err)
			return
		}
		c.JSON(200, meta)
	}
}

func (p *PublicServer) PutHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		putObject(c, store)
	}
}

func (p *PublicServer) DeleteHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		deleteObject(c, store)
	}
}
