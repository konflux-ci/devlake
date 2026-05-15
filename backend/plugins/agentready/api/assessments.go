package api

import (
	"net/http"
	"strconv"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

func GetAssessments(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	page, _ := strconv.Atoi(input.Query.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var clauses []dal.Clause

	if projectName := input.Query.Get("projectName"); projectName != "" {
		clauses = []dal.Clause{
			dal.Select("a.*"),
			dal.From("_tool_agentready_assessments a"),
			dal.Join("JOIN project_mapping pm ON a.repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", projectName, "repos"),
		}
	} else {
		clauses = []dal.Clause{
			dal.From(&models.AgentReadyAssessment{}),
		}
		if repoId := input.Query.Get("repoId"); repoId != "" {
			clauses = append(clauses, dal.Where("repo_id = ?", repoId))
		}
	}

	if cert := input.Query.Get("certification"); cert != "" {
		clauses = append(clauses, dal.Where("certification_level = ?", cert))
	}

	countClauses := make([]dal.Clause, len(clauses))
	copy(countClauses, clauses)
	total, err := db.Count(countClauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to count assessments")
	}

	clauses = append(clauses,
		dal.Orderby("assessed_at DESC"),
		dal.Limit(pageSize),
		dal.Offset(offset),
	)

	var assessments []models.AgentReadyAssessment
	err = db.All(&assessments, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query assessments")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"assessments": assessments,
			"page":        page,
			"pageSize":    pageSize,
			"total":       total,
		},
		Status: http.StatusOK,
	}, nil
}

func GetAssessment(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	id := input.Params["id"]
	var assessment models.AgentReadyAssessment
	err := db.First(&assessment, dal.Where("id = ?", id))
	if err != nil {
		return nil, errors.Default.Wrap(err, "assessment not found")
	}

	var findings []models.AgentReadyFinding
	err = db.All(&findings,
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("assessment_id = ?", id),
		dal.Orderby("tier ASC, status ASC"),
	)

	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query findings for assessment")
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"assessment": assessment,
			"findings":   findings,
		},
		Status: http.StatusOK,
	}, nil
}

func GetAssessmentFindings(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	assessmentId := input.Params["id"]

	clauses := []dal.Clause{
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("assessment_id = ?", assessmentId),
	}

	if tier := input.Query.Get("tier"); tier != "" {
		clauses = append(clauses, dal.Where("tier = ?", tier))
	}
	if status := input.Query.Get("status"); status != "" {
		clauses = append(clauses, dal.Where("status = ?", status))
	}
	if category := input.Query.Get("category"); category != "" {
		clauses = append(clauses, dal.Where("category = ?", category))
	}

	clauses = append(clauses, dal.Orderby("tier ASC, status ASC"))

	var findings []models.AgentReadyFinding
	err := db.All(&findings, clauses...)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query findings")
	}

	return &plugin.ApiResourceOutput{
		Body:   findings,
		Status: http.StatusOK,
	}, nil
}
