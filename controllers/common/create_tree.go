package common

import (
	"context"
	"fmt"
	"github.com/securesign/operator/controllers/common/utils/kubernetes"
	"net"
	"time"

	"github.com/google/trillian"
	"github.com/google/trillian/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/klog/v2"
)

// reference code https://github.com/sigstore/scaffolding/blob/main/cmd/trillian/createtree/main.go
func CreateTrillianTree(ctx context.Context, displayName string, trillianURL string) (*trillian.Tree, error) {
	inContainer, err := kubernetes.ContainerMode()
	if err == nil {
		if !inContainer {
			fmt.Println("Operator is running on localhost. You need to port-forward services.")
			for it := 0; it < 60; it++ {
				if rawConnect("localhost", "8091") {
					fmt.Println("Connection is open.")
					trillianURL = "localhost:8091"
					break
				} else {
					fmt.Println("Execute `oc port-forward service/trillian-logserver 8091 8091` in your namespace to continue.")
					time.Sleep(time.Duration(5) * time.Second)
				}
			}

		}
	} else {
		klog.Info("Can't recognise operator mode - expecting in-container run")
	}

	req, err := newRequest(displayName)
	if err != nil {
		return nil, err
	}
	var opts grpc.DialOption
	klog.Warning("Using an insecure gRPC connection to Trillian")
	opts = grpc.WithTransportCredentials(insecure.NewCredentials())

	conn, err := grpc.Dial(trillianURL, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer conn.Close()

	adminClient := trillian.NewTrillianAdminClient(conn)
	logClient := trillian.NewTrillianLogClient(conn)

	tree, err := client.CreateAndInitTree(ctx, req, adminClient, logClient)

	if err != nil {
		return nil, fmt.Errorf("could not create Trillian tree: %w", err)
	}

	return tree, err
}

func rawConnect(host string, port string) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		defer conn.Close()
		return true
	}
	return false
}

func newRequest(displayName string) (*trillian.CreateTreeRequest, error) {
	ts, ok := trillian.TreeState_value[trillian.TreeState_ACTIVE.String()]
	if !ok {
		return nil, fmt.Errorf("unknown TreeState: %v", trillian.TreeState_ACTIVE)
	}

	tt, ok := trillian.TreeType_value[trillian.TreeType_LOG.String()]
	if !ok {
		return nil, fmt.Errorf("unknown TreeType: %v", trillian.TreeType_LOG)
	}

	ctr := &trillian.CreateTreeRequest{Tree: &trillian.Tree{
		TreeState:       trillian.TreeState(ts),
		TreeType:        trillian.TreeType(tt),
		DisplayName:     displayName,
		MaxRootDuration: durationpb.New(time.Hour),
	}}

	return ctr, nil
}
