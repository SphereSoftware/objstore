package api

import (
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
	r.GET("/api/v1/get", p.GetHandler(store))
	r.POST("/api/v1/put", p.PutHandler(store))
	r.GET("/api/v1/id", p.IDHandler())
	r.GET("/api/v1/version", p.VersionHandler())
	r.GET("/api/v1/ping", p.PingHandler())
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

func (p *PublicServer) GetHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func (p *PublicServer) PutHandler(store objstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
