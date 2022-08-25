package single

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"
	consensus "github.com/umbracle/go-eth-consensus"
	"github.com/umbracle/go-eth-consensus/chaintime"
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

func uintP(i int) *int {
	return &i
}

func (s *SingleDeployment) Run(f *framework.F) {

	config := server.DefaultConfig()
	config.NumGenesisValidators = 10
	config.NumTranches = 1
	config.Spec.MinGenesisValidatorCount = 10
	config.Spec.MinGenesisTime = int(time.Now().Add(10 * time.Second).Unix())
	config.Spec.Altair = uintP(1)

	srv, err := server.NewServer(hclog.NewNullLogger(), config)
	if err != nil {
		panic(err)
	}

	_, err = srv.NodeDeploy(context.Background(), &proto.NodeDeployRequest{
		NodeClient: proto.NodeClient_Prysm,
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

	{
		_, err = srv.NodeDeploy(context.Background(), &proto.NodeDeployRequest{
			NodeClient: proto.NodeClient_Lighthouse,
			NodeType: &proto.NodeDeployRequest_Beacon_{
				Beacon: &proto.NodeDeployRequest_Beacon{
					Count: 1,
				},
			},
		})
		if err != nil {
			panic(err)
		}
		_, err = srv.NodeDeploy(context.Background(), &proto.NodeDeployRequest{
			NodeClient: proto.NodeClient_Teku,
			NodeType: &proto.NodeDeployRequest_Beacon_{
				Beacon: &proto.NodeDeployRequest_Beacon{
					Count: 1,
				},
			},
		})
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(10 * time.Second)

	xx, err := srv.NodeList(context.Background(), &proto.NodeListRequest{})
	if err != nil {
		panic(err)
	}
	beacons := []*proto.Node{}
	for _, i := range xx.Nodes {
		if i.Type == proto.NodeType_Beacon {
			beacons = append(beacons, i)
		}
	}

	var genesisTime uint64

	specs := []*consensus.Spec{}
	for _, node := range beacons {
		client := http.New(node.GetAddr(proto.NodePortHttp))
		spec, err := client.Config().Spec()
		if err != nil {
			panic(err)
		}
		genesis, err := client.Beacon().Genesis()
		if err != nil {
			panic(err)
		}
		genesisTime = genesis.Time
		specs = append(specs, spec)
	}

	for i := 0; i < len(specs)-1; i++ {
		if !compareSpec(specs[i], specs[i+1]) {
			fmt.Println(specs[i], specs[i+1])
			panic("bad")
		}
	}

	genesis := specs[0]
	cTime := chaintime.New(time.Unix(int64(genesisTime), 0).UTC(), genesis.SecondsPerSlot, genesis.SlotsPerEpoch)

	// wait for slot 10
	fmt.Println("time", time.Now(), cTime.Epoch(3).Time)
	fmt.Println(config.Spec.SecondsPerSlot, 10*config.Spec.SecondsPerSlot)

	timer := cTime.Epoch(3).C()
	select {
	case <-timer.C:
		fmt.Println("- epoch 3 -")

		for _, node := range beacons {
			client := http.New(node.GetAddr(proto.NodePortHttp))
			syncing, err := client.Node().Syncing()
			if err != nil {
				panic(err)
			}
			fmt.Println(syncing.HeadSlot, syncing.SyncDistance)
		}
	}

	/*
		fmt.Println("- done -")
		for _, node := range resp.Nodes {
			if node.Type == proto.NodeType_Beacon {
				client := http.New(node.GetAddr(proto.NodePortHttp))
				fmt.Println(client.Node().Identity())
			}
		}
	*/
}

func compareSpec(a, b *consensus.Spec) bool {
	return reflect.DeepEqual(a, b)
}
