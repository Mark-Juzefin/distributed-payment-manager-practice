//go:build integration

package integration_test

import (
	"TestTaskJustPay/pkg/testinfra"
	api "TestTaskJustPay/services/paymanager"
	sg "TestTaskJustPay/services/silvergate"
	"context"
	"fmt"
	"os"
	"testing"
)

// suite holds testcontainer infrastructure (created in TestMain)
var suite *testinfra.TestSuite

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	suite, err = testinfra.NewTestSuite(ctx, testinfra.SuiteOptions{
		WithKafka:             true,
		WithWiremock:          true,
		WithSilvergate:        true,
		MappingsPath:          "mappings",
		WithE2E:               true,
		ProjectRoot:           "..",
		MigrationFS:           api.MIGRATION_FS,
		SilvergateMigrationFS: sg.MigrationFS(),
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to start test suite: %v", err))
	}

	code := m.Run()

	suite.Cleanup(ctx)
	os.Exit(code)
}
