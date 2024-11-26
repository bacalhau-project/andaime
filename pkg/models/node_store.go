package models

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

const defaultNodesFile = "nodes.yml"

// NodeStore handles persistent storage of node information
type NodeStore struct {
	filepath string
	mutex    sync.RWMutex
}

// NewNodeStore creates a new NodeStore instance
func NewNodeStore(filepath string) *NodeStore {
	if filepath == "" {
		filepath = defaultNodesFile
	}
	return &NodeStore{
		filepath: filepath,
	}
}

// AddNode adds a new node to the store
func (s *NodeStore) AddNode(node NodeInfo) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Validate node
	if err := node.Validate(); err != nil {
		return fmt.Errorf("invalid node: %w", err)
	}

	// Read existing nodes
	nodes, err := s.readNodes()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read nodes: %w", err)
	}

	// Check for duplicates
	for _, existing := range nodes.Nodes {
		if existing.Name == node.Name && existing.NodeIP == node.NodeIP {
			return fmt.Errorf("node with name %s and IP %s already exists", node.Name, node.NodeIP)
		}
	}

	// Add new node
	nodes.Nodes = append(nodes.Nodes, node)

	// Write back to file
	return s.writeNodes(nodes)
}

// GetNodes returns all stored nodes
func (s *NodeStore) GetNodes() ([]NodeInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	nodes, err := s.readNodes()
	if err != nil {
		if os.IsNotExist(err) {
			return []NodeInfo{}, nil
		}
		return nil, err
	}

	return nodes.Nodes, nil
}

// readNodes reads the nodes from the YAML file
func (s *NodeStore) readNodes() (*NodeInfoList, error) {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return nil, err
	}

	var nodes NodeInfoList
	if err := yaml.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse nodes file: %w", err)
	}

	return &nodes, nil
}

// writeNodes writes the nodes to the YAML file atomically
func (s *NodeStore) writeNodes(nodes *NodeInfoList) error {
	data, err := yaml.Marshal(nodes)
	if err != nil {
		return fmt.Errorf("failed to marshal nodes: %w", err)
	}

	// Write to temporary file first
	dir := filepath.Dir(s.filepath)
	tmpFile, err := os.CreateTemp(dir, "nodes_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up in case of errors

	if err := os.MkdirAll(filepath.Dir(s.filepath), fs.FileMode(0700)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(tmpFile.Name(), data, fs.FileMode(0600)); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Rename temporary file to actual file (atomic operation)
	if err := os.Rename(tmpFile.Name(), s.filepath); err != nil {
		os.Remove(tmpFile.Name()) // Clean up temp file if rename fails
		return fmt.Errorf("failed to save nodes file: %w", err)
	}

	return nil
}
