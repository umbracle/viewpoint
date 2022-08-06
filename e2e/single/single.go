package single

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/go-eth-consensus/http"
	"github.com/umbracle/viewpoint/e2e/framework"
	"github.com/umbracle/viewpoint/internal/server"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

func init() {
	framework.AddSuites(&framework.TestSuite{
		Cases: []framework.TestCase{new(SingleDeployment)},
	})
}

type SingleDeployment struct {
}

func (s *SingleDeployment) Run(f *framework.F) {
	genesisTime := time.Now().Add(10 * time.Second)

	config := server.DefaultConfig()
	config.NumGenesisValidators = 10
	config.NumTranches = 1
	config.Spec.MinGenesisValidatorCount = 10
	config.Spec.MinGenesisTime = int(genesisTime.Unix())

	srv, err := server.NewServer(hclog.NewNullLogger(), config)
	if err != nil {
		panic(err)
	}

	resp, err := srv.NodeDeploy(context.Background(), &proto.NodeDeployRequest{
		NodeClient: proto.NodeClient_Lighthouse,
		NodeType: &proto.NodeDeployRequest_Validator_{
			Validator: &proto.NodeDeployRequest_Validator{
				NumTranch:   0,
				WithBeacon:  true,
				BeaconCount: 2,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	timer := framework.NewChainTime(genesisTime, uint64(config.Spec.SecondsPerSlot), uint64(config.Spec.SlotsPerEpoch))
	go timer.Run()

	for c := range timer.ResCh {
		if c.Epoch == 2 {
			break
		}
	}

	fmt.Println("- done -")
	for _, node := range resp.Nodes {
		if node.Type == proto.NodeType_Beacon {
			client := http.New(node.GetAddr(proto.NodePortHttp))
			fmt.Println(client.Node().Identity())
		}
	}
}
