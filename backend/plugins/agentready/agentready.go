package main

import (
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/plugins/agentready/impl"
	"github.com/spf13/cobra"
)

var PluginEntry impl.AgentReady

func main() {
	cmd := &cobra.Command{Use: "agentready"}
	connectionId := cmd.Flags().Uint64P("connectionId", "c", 0, "connection ID")
	fullName := cmd.Flags().StringP("fullName", "f", "", "scope full name (org/repo)")

	cmd.Run = func(_ *cobra.Command, args []string) {
		runner.DirectRun(cmd, args, PluginEntry, map[string]any{
			"connectionId": *connectionId,
			"fullName":     *fullName,
		}, "")
	}
	runner.RunCmd(cmd)
}
