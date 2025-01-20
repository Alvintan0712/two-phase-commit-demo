package zkclient

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
)

type ZooKeeperClient struct {
	conn *zk.Conn
}

func NewZooKeeperClient(servers []string) (*ZooKeeperClient, error) {
	conn, _, err := zk.Connect(servers, time.Second)
	if err != nil {
		return nil, err
	}

	return &ZooKeeperClient{conn: conn}, nil
}

func (c *ZooKeeperClient) Close() {
	c.conn.Close()
}

func (c *ZooKeeperClient) Create(path string, data []byte) error {
	_, err := c.conn.Create(path, data, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}

	return nil
}

func (c *ZooKeeperClient) CreateSequential(path string, data []byte) (string, error) {
	nodePath, err := c.conn.Create(path, data, zk.FlagSequence, zk.WorldACL(zk.PermAll))
	if err != nil {
		return "", err
	}

	nodes := strings.Split(nodePath, "/")

	return nodes[len(nodes)-1], nil
}

func (c *ZooKeeperClient) CreateEmphemeral(path string, data []byte) error {
	_, err := c.conn.Create(path, data, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}

	return nil
}

func (c *ZooKeeperClient) CreateProtectedEphemeralSequentialNode(path string, data []byte) (string, error) {
	nodePath, err := c.conn.CreateProtectedEphemeralSequential(path, data, zk.WorldACL(zk.PermAll))
	if err != nil {
		return "", err
	}

	return nodePath, nil
}

func (c *ZooKeeperClient) Set(path string, data []byte) error {
	_, err := c.conn.Set(path, data, -1)
	if err != nil {
		return err
	}

	return nil
}

func (c *ZooKeeperClient) Get(path string) ([]byte, error) {
	data, _, err := c.conn.Get(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *ZooKeeperClient) GetW(path string) ([]byte, <-chan zk.Event, error) {
	data, _, ch, err := c.conn.GetW(path)
	if err != nil {
		return nil, nil, err
	}

	return data, ch, nil
}

func (c *ZooKeeperClient) Exists(path string) (bool, error) {
	exists, _, err := c.conn.Exists(path)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (c *ZooKeeperClient) ExistsW(path string) (bool, <-chan zk.Event, error) {
	exists, _, ch, err := c.conn.ExistsW(path)
	if err != nil {
		return false, nil, err
	}

	return exists, ch, nil
}

func (c *ZooKeeperClient) Children(path string) ([]string, error) {
	children, _, err := c.conn.Children(path)
	if err != nil {
		return nil, err
	}

	return children, nil
}

func (c *ZooKeeperClient) ChildrenW(path string) ([]string, <-chan zk.Event, error) {
	children, _, ch, err := c.conn.ChildrenW(path)
	if err != nil {
		return nil, nil, err
	}

	return children, ch, nil
}

func (c *ZooKeeperClient) Delete(path string) error {
	err := c.conn.Delete(path, -1)
	if err != nil {
		return err
	}

	return nil
}

func (c *ZooKeeperClient) DeleteRecursive(path string) error {
	children, err := c.Children(path)
	if err != nil {
		return fmt.Errorf("error in get children %s: %v", path, err)
	}

	for _, child := range children {
		childPath := path + "/" + child
		err := c.DeleteRecursive(childPath)
		if err != nil {
			return err
		}
	}

	err = c.conn.Delete(path, -1)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %v", path, err)
	}

	return nil
}
