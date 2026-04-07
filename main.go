package main

import (
	"context"
	"fmt"

	"github.com/weka/kubectl-weka/cmd"
	"github.com/weka/kubectl-weka/pkg/config"
	"github.com/weka/kubectl-weka/pkg/logging"
	"github.com/weka/kubectl-weka/pkg/utils"
)

func main() {
	logger := logging.GetLogger(context.Background())
	cleanupCtx := logging.WithLogger(context.Background(), logger)
	defer utils.CleanUpOnExit(cleanupCtx)

	config.Load()
	defer func() {
		err := config.Save()
		if err != nil {
			fmt.Printf("Failed to save configuration: %v", err)
		}
	}()
	cmd.Execute()
}
