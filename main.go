package main

import (
	"context"
	"github.com/weka/kubectl-weka/cmd"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/utils"
)

func main() {
	logger := logging.GetLogger(context.Background())
	cleanupCtx := logging.WithLogger(context.Background(), logger)
	defer utils.CleanUpOnExit(cleanupCtx)

	cmd.Execute()
}
