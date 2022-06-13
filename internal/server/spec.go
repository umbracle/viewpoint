package server

import (
	"encoding"
	"encoding/json"
	"io"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

type Spec struct {
	Repository string
	Tag        string
	Cmd        []string
	Retry      func(n *node) error
	Name       string
	Mount      []string
	Files      map[string][]byte
	Output     []io.Writer
	Labels     map[string]string
	NodeClient proto.NodeClient
	NodeType   proto.NodeType
	User       string
}

func (s *Spec) WithNodeClient(nodeClient proto.NodeClient) *Spec {
	s.NodeClient = nodeClient
	return s
}

func (s *Spec) WithNodeType(nodeType proto.NodeType) *Spec {
	s.NodeType = nodeType
	return s
}

func (s *Spec) WithContainer(repository string) *Spec {
	s.Repository = repository
	return s
}

func (s *Spec) WithTag(tag string) *Spec {
	s.Tag = tag
	return s
}

func (s *Spec) WithCmd(cmd []string) *Spec {
	s.Cmd = append(s.Cmd, cmd...)
	return s
}

func (s *Spec) WithName(name string) *Spec {
	s.Name = name
	return s
}

func (s *Spec) WithRetry(retry func(n *node) error) *Spec {
	s.Retry = retry
	return s
}

func (s *Spec) WithMount(mount string) *Spec {
	s.Mount = append(s.Mount, mount)
	return s
}

func (s *Spec) WithOutput(output io.Writer) *Spec {
	s.Output = append(s.Output, output)
	return s
}

func (s *Spec) WithLabels(m map[string]string) *Spec {
	for k, v := range m {
		s.Labels[k] = v
	}
	return s
}

func (s *Spec) WithUser(user string) *Spec {
	s.User = user
	return s
}

func (s *Spec) WithFile(path string, obj interface{}) *Spec {
	var data []byte
	var err error

	if objS, ok := obj.(string); ok {
		data = []byte(objS)
	} else if objB, ok := obj.([]byte); ok {
		data = objB
	} else if objT, ok := obj.(encoding.TextMarshaler); ok {
		data, err = objT.MarshalText()
	} else {
		data, err = json.Marshal(obj)
	}
	if err != nil {
		panic(err)
	}
	s.Files[path] = data
	return s
}
