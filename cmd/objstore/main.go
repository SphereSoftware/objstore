package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
	"github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	"github.com/xlab/objstore"
	"github.com/xlab/objstore/api"
	"github.com/xlab/objstore/cluster"
	"github.com/xlab/objstore/journal"
	"github.com/xlab/objstore/storage"
)

var app = cli.App("objstore", "Implements robust cache-like object storage on top of S3 backend.")

var (
	debugEnabled bool
	debugLevel   = app.Int(cli.IntOpt{
		Name:      "d debug",
		Desc:      "Debug level to use (0-3)",
		EnvVar:    "APP_DEBUG_LEVEL",
		Value:     0,
		HideValue: true,
	})
	clusterNodes = app.Strings(cli.StringsOpt{
		Name:      "N nodes",
		Desc:      "A list of cluster nodes to join for discovery and journal updates",
		EnvVar:    "APP_CLUSTER_NODES",
		Value:     []string{},
		HideValue: true,
	})
	clusterName = app.String(cli.StringOpt{
		Name:   "T tag",
		Desc:   "Cluster tag name",
		EnvVar: "APP_CLUSTER_TAGNAME",
		Value:  "default",
	})
	privateAddr = app.String(cli.StringOpt{
		Name:   "private-addr",
		Desc:   "Listen address for cluster discovery and private API",
		EnvVar: "NET_PRIVATE_ADDR",
		Value:  "0.0.0.0:11999",
	})
	debugAddr = app.String(cli.StringOpt{
		Name:   "debug-addr",
		Desc:   "Listen address for private API debugging using external tools",
		EnvVar: "NET_DEBUG_ADDR",
		Value:  "0.0.0.0:10080",
	})
	publicAddr = app.String(cli.StringOpt{
		Name:   "public-addr",
		Desc:   "Listen address for external access and public HTTP API",
		EnvVar: "NET_PUBLIC_ADDR",
		Value:  "0.0.0.0:10999",
	})
	statePrefix = app.String(cli.StringOpt{
		Name:   "state-dir",
		Desc:   "Directory where to keep local state and journals.",
		EnvVar: "APP_STATE_DIR",
		Value:  "state/",
	})
	localPrefix = app.String(cli.StringOpt{
		Name:   "files-dir",
		Desc:   "Directory where to keep local files.",
		EnvVar: "APP_FILES_DIR",
		Value:  "files/",
	})
	s3Region = app.String(cli.StringOpt{
		Name:   "R region",
		Desc:   "Amazon S3 region name",
		EnvVar: "S3_REGION_NAME",
		Value:  "us-east-1",
	})
	s3Bucket = app.String(cli.StringOpt{
		Name:   "B bucket",
		Desc:   "Amazon S3 bucket name",
		EnvVar: "S3_BUCKET_NAME",
		Value:  "objstore-stage-1",
	})
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	defer closer.Close()

	closer.Bind(func() {
		runtime.GC()
		log.Println("bye!")
	})

	app.Action = appMain
	app.Before = func() {
		if *debugLevel > 0 {
			debugEnabled = true
		}
		if debugEnabled {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
	}
	if err := app.Run(os.Args); err != nil {
		closer.Fatalln(err)
	}
}

func appMain() {
	db, err := openStateDB(*statePrefix)
	if err != nil {
		closer.Fatalln("[ERR] failed to open state DB:", err)
	}
	if err := os.MkdirAll(*localPrefix, 0700); err != nil {
		closer.Fatalln("[ERR] unable to create local files dir:", err)
	}

	nodeID := journal.PseudoUUID()
	if debugEnabled {
		log.Println("[INFO] node ID", nodeID)
	}

	privateServer := api.NewPrivateServer(nodeID, *clusterName)
	privateServer.SetDebug(debugEnabled)
	privateClient := cluster.NewPrivateClient(privateServer.Router())
	journalManager := journal.NewJournalManager(db)
	closer.Bind(func() {
		if err := journalManager.Close(); err != nil {
			log.Println("[WARN] journal close:", err)
		}
	})
	store, err := objstore.NewStore(nodeID,
		storage.NewLocalStorage(*localPrefix),
		storage.NewS3Storage(*s3Region, *s3Bucket),
		journalManager,
		cluster.NewClusterManager(privateClient, nodeID),
	)
	if err != nil {
		closer.Fatalln("[ERR]", err)
	}
	store.SetDebug(debugEnabled)
	privateServer.RouteAPI(store)
	if err := privateServer.ListenAndServe(*privateAddr); err != nil {
		closer.Fatalln(err)
	}

	closer.Bind(func() {
		if err := store.Close(); err != nil {
			log.Println("[WARN]", err)
		}
		if debugEnabled {
			log.Println("[INFO] waiting for queues")
		}
		wg := new(sync.WaitGroup)
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.WaitInbound(2 * time.Minute)
		}()
		go func() {
			defer wg.Done()
			store.WaitOutbound(2 * time.Minute)
		}()
		wg.Wait()
	})

	if len(*clusterNodes) == 0 {
		log.Println("[WARN] no additional cluster nodes specified, current node starts solo")
	} else {
		if debugEnabled {
			log.Println("[INFO] joining to cluster", *clusterNodes)
		}
		if err := privateServer.JoinCluster(*clusterNodes); err != nil {
			log.Println("[WARN]", err)
		}
	}
	// expose private API to HTTP clients, so objstore cluster nodes can be debugged
	// using browser and external tools.
	if debugEnabled {
		log.Println("[INFO] exposing private API on", *debugAddr)
		go func() {
			if err := privateServer.ExposeAPI(*debugAddr); err != nil {
				closer.Fatalln("[ERR]", err)
			}
		}()
	}

	publicServer := api.NewPublicServer(nodeID)
	publicServer.RouteAPI(store)
	go func() {
		if err := publicServer.ListenAndServe(*publicAddr); err != nil {
			closer.Fatalln(err)
		}
	}()

	closer.Hold()
}

func openStateDB(prefix string) (*bolt.DB, error) {
	if err := os.MkdirAll(prefix, 0700); err != nil {
		return nil, err
	}
	return bolt.Open(filepath.Join(prefix, "state.db"), 0600, &bolt.Options{
		Timeout:         30 * time.Second,       // wait while trying to open state file
		InitialMmapSize: 4 * 1024 * 1024 * 1024, // preallocated space to avoid writers block
	})
}
