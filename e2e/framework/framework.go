package framework

import "fmt"

var pkgSuites []*TestSuite

func AddSuites(s *TestSuite) {
	pkgSuites = append(pkgSuites, s)
}

type TestSuite struct {
	Cases []TestCase
}

type TestCase interface {
	Run(f *F)
}

type Framework struct {
	suites []*TestSuite
}

func New() *Framework {
	f := &Framework{
		suites: pkgSuites,
	}
	return f
}

func (f *Framework) Run() {
	fmt.Println(f.suites, pkgSuites)
	for _, s := range f.suites {
		for _, c := range s.Cases {
			f := &F{}
			c.Run(f)
		}
	}
}
