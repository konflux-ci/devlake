package api

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/plugin"
)

var db dal.Dal
var basicRes context.BasicRes

func Init(br context.BasicRes, _ plugin.PluginMeta) {
	basicRes = br
	db = basicRes.GetDal()
}
