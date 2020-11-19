// +build integration

package example

import (
	"context"
	"os"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
)

func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestExample(t *testing.T) {
	needs := base.ClusterNeeds{
		NamespaceId:     "example",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	testRunner := &ExampleTestRunner{}
	testRunner.BuildOrSkip(t, needs, nil)
	ctx, cancel := context.WithCancel(context.Background())
	base.HandleInterruptSignal(t, func(t *testing.T) {
		testRunner.TearDown(ctx)
		cancel()
	})
	testRunner.Run(ctx, t)
}
