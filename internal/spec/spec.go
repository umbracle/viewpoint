package spec

import (
	"encoding"
	"encoding/json"
	"io"
)

type Node interface {
	GetAddr(port string) string
	GetLogs() (string, error)
	Spec() *Spec
	Stop() error
}

type Spec struct {
	Repository string
	Tag        string
	Cmd        []string
	Retry      func(n Node) error
	Name       string
	Mount      []string
	Files      map[string][]byte
	Output     []io.Writer
	Labels     map[string]string
	User       string
}

func (s *Spec) HasLabel(k, v string) bool {
	return s.Labels[k] == v
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
	if len(s.Cmd) == 0 {
		s.Cmd = []string{}
	}
	s.Cmd = append(s.Cmd, cmd...)
	return s
}

func (s *Spec) WithName(name string) *Spec {
	s.Name = name
	return s
}

func (s *Spec) WithRetry(retry func(n Node) error) *Spec {
	s.Retry = retry
	return s
}

func (s *Spec) WithMount(mount string) *Spec {
	if len(s.Mount) == 0 {
		s.Mount = []string{}
	}
	s.Mount = append(s.Mount, mount)
	return s
}

func (s *Spec) WithOutput(output io.Writer) *Spec {
	if len(s.Output) == 0 {
		s.Output = []io.Writer{}
	}
	s.Output = append(s.Output, output)
	return s
}

func (s *Spec) WithLabel(k, v string) *Spec {
	if len(s.Labels) == 0 {
		s.Labels = map[string]string{}
	}
	s.Labels[k] = v
	return s
}

func (s *Spec) WithLabels(m map[string]string) *Spec {
	for k, v := range m {
		s.WithLabel(k, v)
	}
	return s
}

func (s *Spec) WithUser(user string) *Spec {
	s.User = user
	return s
}

func (s *Spec) WithFile(path string, obj interface{}) *Spec {
	if len(s.Files) == 0 {
		s.Files = map[string][]byte{}
	}

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
