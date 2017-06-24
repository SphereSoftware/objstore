package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
)

type ClusterManager interface {
	ListNodes() ([]*NodeInfo, error)
	Announce(nodeID string, event *EventAnnounce) error
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

func (c *clusterManager) Announce(nodeID string, event *EventAnnounce) error {
	body, _ := json.Marshal(event)
	resp, err := c.cli.POST(nodeID, "/private/v1/announce", bytes.NewReader(body))
	if err != nil {
		return err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(string(respBody))
	}
	return nil
}
