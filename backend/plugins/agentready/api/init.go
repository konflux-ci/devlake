package api

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

var basicRes context.BasicRes

var dsHelper *api.DsHelper[models.AgentReadyConnection, models.AgentReadyScope, models.AgentReadyScopeConfig]

func Init(br context.BasicRes, p plugin.PluginMeta) {
	basicRes = br
	dsHelper = api.NewDataSourceHelper[
		models.AgentReadyConnection, models.AgentReadyScope, models.AgentReadyScopeConfig,
	](
		br,
		p.Name(),
		[]string{"fullName"},
		func(c models.AgentReadyConnection) models.AgentReadyConnection {
			return c.Sanitize()
		},
		nil,
		nil,
	)
}
