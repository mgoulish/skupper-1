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

var verbose    bool   = true
var red        string = "\033[1;31m"
var green      string = "\033[1;32m"
var cyan       string = "\033[1;36m"
var yellow     string = "\033[1;33m"
var resetColor string = "\033[0m"

type ExampleTestRunner struct {
	base.ClusterTestRunnerBase
}


type TestCase struct {
        name               string
        diagram            []string
	createOptsPublic   types.SiteConfigSpec
	createOptsPrivate  types.SiteConfigSpec
        public_public_cnx  map[int]int
        private_public_cnx []int
        direct_count       int
        indirect_count     int
}


func (r *ExampleTestRunner) RunTests(testCase * TestCase, ctx context.Context, t *testing.T) ( err error ) {

	pubCluster, err := r.GetPublicContext(1)
	assert.Assert(t, err)

	prvCluster, err := r.GetPrivateContext(1)
	assert.Assert(t, err)

	tick := time.Tick(constants.DefaultTick)
	timeout := time.After(constants.ImagePullingAndResourceCreationTimeout)
	wait_for_conn := func(cc *base.ClusterContext, countConnections bool) (err error) {
		for {
			select {
			case <-ctx.Done():
				t.Logf("context has been canceled")
				t.FailNow()
			case <-timeout:
				assert.Assert(t, false, "Timeout waiting for connection")
			case <-tick:
				vir, err := cc.VanClient.RouterInspect(ctx)
                                // TODO : THIS HAS TO CHANGE
                                        fp ( os.Stdout, "MDEBUG  vir.Status.ConnectedSites : |%#v|\n", vir.Status.ConnectedSites )
				if err == nil && vir.Status.ConnectedSites.Total >= 1 {
					t.Logf("Van sites connected!\n")
                                        if err != nil {
                                          return err
                                        }
                                        if ! countConnections {
                                          fp ( os.Stdout, "MDEBUG I am NOT counting connections.\n" )
                                          return nil
                                        }
                                        fp ( os.Stdout, "MDEBUG I am counting connections.\n")
                                        fp ( os.Stdout, "MDEBUG what am I expecting ??? : %d %d\n", testCase.direct_count, testCase.indirect_count )
                                        if testCase.direct_count   == vir.Status.ConnectedSites.Direct    && 
                                           testCase.indirect_count == vir.Status.ConnectedSites.Indirect {
                                          return nil
                                        }
					//return
				} else {
					fmt.Printf("Connection not ready yet, current pods state: \n")
					pubCluster.KubectlExec("get pods -o wide")
				}
			}
		}
	}
	err = wait_for_conn(pubCluster, false)
        if err != nil {
          return err
        }
          
	err =wait_for_conn(prvCluster, true)
        if err != nil {
          return err
        }

  return nil
}

func ( r *ExampleTestRunner ) Setup ( ctx context.Context, testCase * TestCase, t * testing.T ) {

        publicSecrets  := make ( map[int]string, 0 )

	// Public ---------------------------------------------
	createOptsPublic := testCase.createOptsPublic
        for i := 0; i < int ( createOptsPublic.Replicas ); i ++ {

          // BUGALERT !!!! NOT '1' !!!   i+1 ???
          pub1Cluster, err := r.GetPublicContext(i+1) // These numbers are 1-based.
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
          // In this test there is always a single private namespace, 
          // and it is always an edge.
          privateCluster, err := r.GetPrivateContext(1) // There is always only 1 private/edge namespace.
          assert.Assert(t, err)

          err = privateCluster.CreateNamespace()
          assert.Assert(t, err)

          testCase.createOptsPrivate.SkupperNamespace = privateCluster.Namespace
          siteConfig, err := privateCluster.VanClient.SiteConfigCreate(context.Background(), testCase.createOptsPrivate)
          assert.Assert(t, err)
          err = privateCluster.VanClient.RouterCreate(ctx, *siteConfig)
          assert.Assert(t, err)

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
        for _, public := range ( testCase.private_public_cnx ) {
          secretFileName := publicSecrets [ public-1 ]
          privateCluster, err := r.GetPrivateContext(1)
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
        // In this test there is always one private namespace,
        // and it is always an edge.
	testcases := []TestCase{
		{
                        name: "one-direct",
                        diagram: []string{"edge  -->  interior"},
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
				IsEdge:            true,
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
                        // The IDs on clusters are 1-based, not 0-based.
                        private_public_cnx : []int { 1 },
                        direct_count: 1,
                        indirect_count: 0,
		},
	}

	defer r.TearDown(ctx)


	for test_index, test := range testcases {
		t.Logf("Testing: %s\n", test.name)
                if verbose {
                  fp(os.Stdout, "\n\n%stest %d: %s%s%s\n", yellow, test_index+1, cyan, test.name, resetColor)
                  fp(os.Stdout, "%s", cyan)
                  for _, s := range test.diagram {
                    fp(os.Stdout, "\t%s\n", s)
                  }
                  fp(os.Stdout, "\n\tdirect: %d   indirect: %d\n", test.direct_count, test.indirect_count)
                  fp(os.Stdout, "%s\n\n", resetColor)
                }
		r.Setup ( ctx, & test, t )
		err := r.RunTests(& test, ctx, t)
		r.TearDown(ctx)
                assert.Assert(t, err)
	}
}
