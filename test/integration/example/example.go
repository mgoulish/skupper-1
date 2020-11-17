package example

import (
	"context"
	"fmt"
        "os"
	"testing"
	"time"

	"github.com/prometheus/common/log"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

var fp = fmt.Fprintf

type ExampleTestRunner struct {
	base.ClusterTestRunnerBase
}


type TestCase struct {
	doc                string
	createOptsPublic   types.SiteConfigSpec
	createOptsPrivate  types.SiteConfigSpec
        public_public_cnx  map[int]int
        private_public_cnx map[int]int
}

func (r *ExampleTestRunner) RunTests(ctx context.Context, t *testing.T) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *base.ClusterContext) {
		for {
			select {
			case <-ctx.Done():
				t.Logf("context has been canceled")
				t.FailNow()
			case <-timeout:
				assert.Assert(t, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
				if err == nil && vir.Status.ConnectedSites.Total == 1 {
					t.Logf("Van sites connected!\n")
					return
				} else {
					fmt.Printf("Connection not ready yet, current pods state: \n")
					pubCluster.KubectlExec("get pods -o wide")
				}
			}
		}
	}
	wait_for_conn(pubCluster)
	wait_for_conn(prvCluster)
}

func ( r *ExampleTestRunner ) Setup ( ctx context.Context, testCase * TestCase, t * testing.T ) {

        publicSecrets  := make ( map[int]string, 0 )

	// Public ---------------------------------------------
	createOptsPublic := testCase.createOptsPublic
        for i := 0; i < int ( createOptsPublic.Replicas ); i ++ {

          pub1Cluster, err := r.GetPublicContext(1)
          assert.Assert(t, err)

          err = pub1Cluster.CreateNamespace()
          assert.Assert(t, err)

          // Create and configure the cluster.
          createOptsPublic.SkupperNamespace = pub1Cluster.Namespace
          siteConfig, err := pub1Cluster.VanClient.SiteConfigCreate(context.Background(), createOptsPublic)
          assert.Assert(t, err)

          // Create the router.
          err = pub1Cluster.VanClient.RouterCreate(ctx, *siteConfig)
          assert.Assert(t, err)

          // Create a connection token for this cluster.
          // It is only the public clusters that get connected to.
          // We do this for every public cluster because we are too lazy 
          // to figure out which ones will actually need it.
          secretFileName := fmt.Sprintf("/tmp/public_example_%d_secret.yaml", i+1)
          err = pub1Cluster.VanClient.ConnectorTokenCreateFile(ctx, types.DefaultVanName, secretFileName)
          assert.Assert(t, err)
          publicSecrets[i] = secretFileName
        }

        // Make all public-to-public connections.
        for key, value := range ( testCase.public_public_cnx ) {
          fp ( os.Stdout, "MDEBUG connect public %d to public %d\n", key, value )
        }

	// Private ---------------------------------------------
	createOptsPrivate := testCase.createOptsPrivate
	for i := 0; i < int(createOptsPrivate.Replicas); i++ {
		privateCluster, err := r.GetPrivateContext(i + 1)
		assert.Assert(t, err)

		err = privateCluster.CreateNamespace()
		assert.Assert(t, err)

		createOptsPrivate.SkupperNamespace = privateCluster.Namespace
		siteConfig, err := privateCluster.VanClient.SiteConfigCreate(context.Background(), createOptsPrivate)
		assert.Assert(t, err)
		err = privateCluster.VanClient.RouterCreate(ctx, *siteConfig)
		assert.Assert(t, err)
	}

        // Make all public-to-public connections.
        for public_1, public_2 := range ( testCase.public_public_cnx ) {
          secretFileName := publicSecrets [ public_2-1 ]
          public_1_cluster, err := r.GetPrivateContext(public_1)
          assert.Assert(t, err)
          connectorCreateOpts := types.ConnectorCreateOptions { SkupperNamespace: public_1_cluster.Namespace,
                                                                Name:             "",
                                                                Cost:             0,
                                                              }
          _, err = public_1_cluster.VanClient.ConnectorCreateFromFile(ctx, secretFileName, connectorCreateOpts)
          assert.Assert(t, err)
          fp ( os.Stdout, "MDEBUG connected public_1 |%s| to public_2 |%s|\n", public_1_cluster.Namespace, secretFileName )
        }

        // Make all private-to-public connections.
        for private, public := range ( testCase.private_public_cnx ) {
          secretFileName := publicSecrets [ public-1 ]
          privateCluster, err := r.GetPrivateContext(private)
          assert.Assert(t, err)
          connectorCreateOpts := types.ConnectorCreateOptions { SkupperNamespace: privateCluster.Namespace,
                                                                Name:             "",
                                                                Cost:             0,
                                                              }
          _, err = privateCluster.VanClient.ConnectorCreateFromFile(ctx, secretFileName, connectorCreateOpts)
          assert.Assert(t, err)
          fp ( os.Stdout, "MDEBUG connected private |%s| to public |%s|\n", privateCluster.Namespace, secretFileName )
        }
}

func (r *ExampleTestRunner) TearDown(ctx context.Context) {
	errMsg := "Something failed! aborting teardown"

	pub, err := r.GetPublicContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	priv, err := r.GetPrivateContext(1)
	if err != nil {
		log.Warn(errMsg)
	}

	pub.DeleteNamespace()
	priv.DeleteNamespace()
}

func (r *ExampleTestRunner) Run(ctx context.Context, t *testing.T) {
	testcases := []TestCase{
		{
			doc: "Connecting, two internals, clusterLocal=true",
			createOptsPublic: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				ClusterLocal:      true,
				Replicas:          1,
			},
			createOptsPrivate: types.SiteConfigSpec{
				SkupperName:       "",
				IsEdge:            false,
				EnableController:  true,
				EnableServiceSync: true,
				EnableConsole:     false,
				AuthMode:          types.ConsoleAuthModeUnsecured,
				User:              "nicob?",
				Password:          "nopasswordd",
				ClusterLocal:      true,
				Replicas:          1,
			},
                        public_public_cnx : map[int]int {},
                        private_public_cnx : map[int]int { 1 : 1 },
		},
	}

	defer r.TearDown(ctx)

	for _, c := range testcases {
		t.Logf("Testing: %s\n", c.doc)
		r.Setup(ctx, &c, t)
		r.RunTests(ctx, t)
		r.TearDown(ctx)
	}
}
