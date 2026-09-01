package noderegistry

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"sync"
)

type NodeType string

const (
	NodeTrain    NodeType = "TRAIN"
	NodeDatabase NodeType = "DATA"
)

type Node struct {
	Url  string
	Type NodeType
}

type Registry interface {
	Add(url string, t NodeType)
	Remove(url string)
	GetAll() []Node
	Load() error
	Save() error
}

type DefaultRegistry struct {
	nodes map[string]NodeType
	mux   *sync.RWMutex
	path  string
}

func NewDefaultRegistry(path string) *DefaultRegistry {
	return &DefaultRegistry{
		nodes: make(map[string]NodeType),
		mux:   &sync.RWMutex{},
		path:  path,
	}
}

func (r *DefaultRegistry) Add(url string, t NodeType) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.nodes[url] = t
}

func (r *DefaultRegistry) Remove(url string) {
	r.mux.Lock()
	defer r.mux.Unlock()
	delete(r.nodes, url)
}

func (r *DefaultRegistry) GetAll() []Node {
	r.mux.RLock()
	defer r.mux.RUnlock()
	nodes := make([]Node, 0, len(r.nodes))
	for url, t := range r.nodes {
		nodes = append(nodes, Node{Url: url, Type: t})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Url < nodes[j].Url
	})
	return nodes
}

func (r *DefaultRegistry) Load() error {
	file, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	r.mux.Lock()
	defer r.mux.Unlock()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		r.nodes[parts[1]] = NodeType(parts[0])
	}
	return scanner.Err()
}

func (r *DefaultRegistry) Save() error {
	r.mux.RLock()
	nodes := make([]Node, 0, len(r.nodes))
	for url, t := range r.nodes {
		nodes = append(nodes, Node{Url: url, Type: t})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Url < nodes[j].Url
	})
	r.mux.RUnlock()

	file, err := os.Create(r.path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, n := range nodes {
		if _, err := writer.WriteString(string(n.Type) + "\t" + n.Url + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}
