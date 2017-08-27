package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"sphere.software/objstore/journal"
)

type ClusterManager interface {
	ListNodes() ([]*NodeInfo, error)
	Announce(ctx context.Context, nodeID string, event *EventAnnounce) error
	GetObject(ctx context.Context, nodeID string, id string) (io.ReadCloser, error)
	Sync(ctx context.Context, nodeID string,
		list journal.FileMetaList) (added, deleted journal.FileMetaList, err error)
}

type NodeInfo struct {
	ID    string `json:"id"`
	Addr  string `json:"addr"`
	VAddr string `json:"vaddr"`
}

func NewClusterManager(cli *PrivateClient, nodeID string) ClusterManager {
	return &clusterManager{
		cli:    cli,
		nodeID: nodeID,
	}
}

type clusterManager struct {
	nodeID string
	cli    *PrivateClient
}

func (c *clusterManager) ListNodes() ([]*NodeInfo, error) {
	var nodes []*NodeInfo
	if err := c.cli.ForEachNode(func(id, addr, vaddr string) error {
		nodes = append(nodes, &NodeInfo{
			ID:    id,
			Addr:  addr,
			VAddr: vaddr,
		})
		return nil
	}); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (c *clusterManager) Announce(ctx context.Context, nodeID string, event *EventAnnounce) error {
	body, _ := json.Marshal(event)
	resp, err := c.cli.POST(ctx, nodeID, "/private/v1/announce", bytes.NewReader(body))
	if err != nil {
		return err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		if len(respBody) > 0 {
			err := errors.New(string(respBody))
			return err
		}
		return errors.New(resp.Status)
	}
	return nil
}

var ErrNotFound = errors.New("not found")

func (c *clusterManager) GetObject(ctx context.Context, nodeID string, id string) (io.ReadCloser, error) {
	resp, err := c.cli.GET(ctx, nodeID, "/private/v1/get/"+id, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		resp.Body.Close()
		return nil, ErrNotFound
	} else if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if len(respBody) > 0 {
			err := errors.New(string(respBody))
			return nil, err
		}
		err := errors.New(resp.Status)
		return nil, err
	}
	return resp.Body, nil
}

type SyncResponse struct {
	Added   journal.FileMetaList `json:"list_added"`
	Deleted journal.FileMetaList `json:"list_deleted"`
}

func (c *clusterManager) Sync(ctx context.Context, nodeID string,
	list journal.FileMetaList) (added, deleted journal.FileMetaList, err error) {

	body, _ := json.Marshal(list)
	resp, err := c.cli.POST(ctx, nodeID, "/private/v1/sync", bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		if len(respBody) > 0 {
			err := errors.New(string(respBody))
			return nil, nil, err
		}
		return nil, nil, errors.New(resp.Status)
	}
	var syncResp SyncResponse
	if err := json.Unmarshal(respBody, &syncResp); err != nil {
		return nil, nil, err
	}
	return syncResp.Added, syncResp.Deleted, nil
}
