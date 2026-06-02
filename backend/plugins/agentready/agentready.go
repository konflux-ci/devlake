package main

import (
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/plugins/agentready/impl"
	"github.com/spf13/cobra"
)

var PluginEntry impl.AgentReady

func main() {
	cmd := &cobra.Command{Use: "agentready"}
	projectName := cmd.Flags().StringP("project", "p", "", "project name to analyze")
	repoId := cmd.Flags().StringP("repoId", "r", "", "single repository domain ID")

	cmd.Run = func(_ *cobra.Command, args []string) {
		runner.DirectRun(cmd, args, PluginEntry, map[string]interface{}{
			"projectName": *projectName,
			"repoId":      *repoId,
		}, "")
	}
	runner.RunCmd(cmd)
}
