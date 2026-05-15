package api

import (
	"net/http"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

type AggResult struct {
	Total    int64   `gorm:"column:total"`
	AvgScore float64 `gorm:"column:avg_score"`
}

type CertCount struct {
	CertificationLevel string `gorm:"column:certification_level" json:"certificationLevel"`
	Count              int64  `gorm:"column:count" json:"count"`
}

func GetStats(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var baseClauses []dal.Clause

	if projectName := input.Query.Get("projectName"); projectName != "" {
		baseClauses = []dal.Clause{
			dal.From("_tool_agentready_assessments a"),
			dal.Join("JOIN project_mapping pm ON a.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", projectName, "repos"),
		}
	} else {
		baseClauses = []dal.Clause{
			dal.From(&models.AgentReadyAssessment{}),
		}
	}
	baseClauses = append(baseClauses, dal.Where("id != ''"))

	aggClauses := make([]dal.Clause, len(baseClauses))
	copy(aggClauses, baseClauses)
	aggClauses = append(aggClauses,
		dal.Select("COUNT(*) as total, AVG(overall_score) as avg_score"),
	)

	var agg AggResult
	err := db.First(&agg, aggClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get assessment stats")
	}

	var certCounts []CertCount
	certClauses := make([]dal.Clause, len(baseClauses))
	copy(certClauses, baseClauses)
	certClauses = append(certClauses,
		dal.Select("certification_level, COUNT(*) as count"),
		dal.Groupby("certification_level"),
	)
	err = db.All(&certCounts, certClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to get certification distribution")
	}

	certDist := make(map[string]int, len(certCounts))
	for _, c := range certCounts {
		certDist[c.CertificationLevel] = int(c.Count)
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"totalAssessments":          agg.Total,
			"averageScore":              agg.AvgScore,
			"certificationDistribution": certDist,
		},
		Status: http.StatusOK,
	}, nil
}
