package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

type exitResult struct {
	err error
}

type node struct {
	cli        *client.Client
	id         string
	opts       *Spec
	ip         string
	ports      map[NodePort]uint64
	waitCh     chan struct{}
	exitResult *exitResult
	mountMap   map[string]string
}

type Docker struct {
	cli    *client.Client
	logger hclog.Logger
}

func NewDocker() (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}
	d := &Docker{
		cli:    cli,
		logger: hclog.L(),
	}
	return d, nil
}

func (d *Docker) SetLogger(logger hclog.Logger) {
	d.logger = logger
}

func (d *Docker) Deploy(spec *Spec) (*node, error) {
	ctx := context.Background()

	if spec.Tag == "" {
		spec.Tag = "latest"
	}

	// setup configuration
	dirPrefix := "node-"
	if spec.Name != "" {
		dirPrefix += spec.Name + "-"
	}

	// build any mount path
	mountMap := map[string]string{}
	for _, mount := range spec.Mount {
		tmpDir, err := ioutil.TempDir("/tmp", dirPrefix)
		if err != nil {
			return nil, err
		}
		mountMap[mount] = tmpDir
	}

	// build the files
	for path, content := range spec.Files {
		// find the mount match
		var mount, local string
		var found bool

	MOUNT:
		for mount, local = range mountMap {
			if strings.HasPrefix(path, mount) {
				found = true
				break MOUNT
			}
		}
		if !found {
			return nil, fmt.Errorf("mount match for '%s' not found", path)
		}

		relPath := strings.TrimPrefix(path, mount)
		localPath := filepath.Join(local, relPath)

		// create all the directory paths required
		parentDir := filepath.Dir(localPath)
		if err := os.MkdirAll(parentDir, 0700); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(localPath, content, 0644); err != nil {
			return nil, err
		}
	}

	imageName := spec.Repository + ":" + spec.Tag

	// pull image if it does not exists
	_, _, err := d.cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		reader, err := d.cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(d.logger.StandardWriter(&hclog.StandardLoggerOptions{}), reader)
		if err != nil {
			return nil, err
		}
	}

	n := &node{
		cli:      d.cli,
		opts:     spec,
		ip:       "127.0.0.1",
		ports:    map[NodePort]uint64{},
		waitCh:   make(chan struct{}),
		mountMap: mountMap,
	}

	// build CLI arguments which might include template arguments
	cmdArgs := []string{}
	for _, cmd := range spec.Cmd {
		cleanCmd, err := n.execCmd(cmd)
		if err != nil {
			return nil, err
		}
		cmdArgs = append(cmdArgs, cleanCmd)
	}

	config := &container.Config{
		Image:  imageName,
		Cmd:    strslice.StrSlice(cmdArgs),
		Labels: spec.Labels,
		User:   spec.User,
	}
	hostConfig := &container.HostConfig{
		Binds:       []string{},
		NetworkMode: "host",
	}

	for mount, local := range mountMap {
		hostConfig.Binds = append(hostConfig.Binds, local+":"+mount)
	}

	body, err := d.cli.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return nil, fmt.Errorf("could not create container: %v", err)
	}
	n.id = body.ID

	// start container
	if err := d.cli.ContainerStart(ctx, n.id, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("could not start container: %v", err)
	}

	go n.run()

	if len(spec.Output) != 0 {
		// track the logs to output
		go func() {
			if err := n.trackOutput(); err != nil {
				d.logger.Error("failed to log container", "id", n.id, "err", err)
			}
		}()
	}

	if spec.Retry != nil {
		if err := n.retryFn(defaultTimeoutDuration, func() error {
			return spec.Retry(n)
		}); err != nil {
			return nil, err
		}
	}

	return n, nil
}

type NodePort string

const (
	// NodePortEth1Http is the http port for the eth1 node.
	NodePortEth1Http = "eth1.http"

	// NodePortP2P is the p2p port for an eth2 node.
	NodePortP2P = "eth2.p2p"

	// NodePortHttp is the http port for an eth2 node.
	NodePortHttp = "eth2.http"

	// NodePortPrysmGrpc is the specific prysm port for its Grpc server
	NodePortPrysmGrpc = "eth2.prysm.grpc"
)

func uintPtr(i uint64) *uint64 {
	return &i
}

// port ranges for each node value.
var ports = map[NodePort]*uint64{
	NodePortEth1Http:  uintPtr(8000),
	NodePortP2P:       uintPtr(5000),
	NodePortHttp:      uintPtr(7000),
	NodePortPrysmGrpc: uintPtr(4000),
}

func (n *node) execCmd(cmd string) (string, error) {
	t := template.New("node_cmd")
	t.Funcs(template.FuncMap{
		"Port": func(name NodePort) string {
			port, ok := ports[name]
			if !ok {
				panic(fmt.Sprintf("Port '%s' not found", name))
			}
			var relPort uint64
			if foundPort, ok := n.ports[name]; ok {
				relPort = foundPort
			} else {
				relPort = atomic.AddUint64(port, 1)
				n.ports[name] = relPort
			}
			return fmt.Sprintf("%d", relPort)
		},
	})

	t, err := t.Parse(cmd)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (n *node) WaitCh() <-chan struct{} {
	return n.waitCh
}

func (n *node) run() {
	resCh, errCh := n.cli.ContainerWait(context.Background(), n.id, container.WaitConditionNotRunning)

	var exitErr error
	select {
	case res := <-resCh:
		if res.Error != nil {
			exitErr = fmt.Errorf(res.Error.Message)
		}
	case err := <-errCh:
		exitErr = err
	}

	n.exitResult = &exitResult{
		err: exitErr,
	}
	close(n.waitCh)
}

func (n *node) GetAddr(port NodePort) string {
	num, ok := n.ports[port]
	if !ok {
		panic(fmt.Sprintf("port '%s' not found", port))
	}
	return fmt.Sprintf("http://%s:%d", n.ip, num)
}

func (n *node) Stop() error {
	if err := n.cli.ContainerStop(context.Background(), n.id, nil); err != nil {
		return fmt.Errorf("failed to stop container: %v", err)
	}
	return nil
}

func (n *node) trackOutput() error {
	writer := io.MultiWriter(n.opts.Output...)

	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}
	out, err := n.cli.ContainerLogs(context.Background(), n.id, opts)
	if err != nil {
		return err
	}
	if _, err := stdcopy.StdCopy(writer, writer, out); err != nil {
		return err
	}
	return nil
}

func (n *node) GetLogs() (string, error) {
	wr := bytes.NewBuffer(nil)

	out, err := n.cli.ContainerLogs(context.Background(), n.id, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", nil
	}
	if _, err := stdcopy.StdCopy(wr, wr, out); err != nil {
		return "", err
	}
	logs := wr.String()
	return logs, nil
}

func (n *node) IP() string {
	return n.ip
}

func (n *node) Type() proto.NodeClient {
	return n.opts.NodeClient
}

var defaultTimeoutDuration = 1 * time.Minute

func (n *node) retryFn(timeout time.Duration, handler func() error) error {
	timeoutT := time.NewTimer(timeout)

	for {
		select {
		case <-time.After(5 * time.Second):
			if err := handler(); err == nil {
				return nil
			}

		case <-n.waitCh:
			return fmt.Errorf("node stopped")

		case <-timeoutT.C:
			return fmt.Errorf("timeout")
		}
	}
}
