package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

var CalculateMetricsMeta = plugin.SubTaskMeta{
	Name:             "calculateMetrics",
	EntryPoint:       CalculateMetrics,
	EnabledByDefault: true,
	Description:      "Compute aggregated pass rates and category scores per assessment",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAssessmentsMeta},
}

func CalculateMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	data := taskCtx.GetData().(*AgentReadyTaskData)
	repos, err := discoverRepos(db, data.Options, logger)
	if err != nil {
		return errors.Default.Wrap(err, "failed to discover repos for metrics calculation")
	}
	if len(repos) == 0 {
		logger.Info("No repos found for metrics calculation, skipping")
		return nil
	}
	repoIds := make([]string, 0, len(repos))
	for _, r := range repos {
		repoIds = append(repoIds, r.DomainRepoId)
	}
	var assessments []models.AgentReadyAssessment
	clauses := []dal.Clause{
		dal.From(&models.AgentReadyAssessment{}),
		dal.Where("repo_id IN (?)", repoIds),
	}

	err = db.All(&assessments, clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query assessments")
	}

	if len(assessments) == 0 {
		logger.Info("No assessments found, skipping metrics")
		return nil
	}

	logger.Info("Calculating metrics for %d assessments", len(assessments))
	taskCtx.SetProgress(0, len(assessments))

	var allFindings []*models.AgentReadyFinding
	assessmentIds := make([]string, 0, len(assessments))
	for _, a := range assessments {
		assessmentIds = append(assessmentIds, a.Id)
	}
	err = db.All(&allFindings,
		dal.From(&models.AgentReadyFinding{}),
		dal.Where("assessment_id IN (?)", assessmentIds),
	)

	if err != nil {
		return errors.Default.Wrap(err, "failed to query findings")
	}

	findingsByAssessment := map[string][]*models.AgentReadyFinding{}

	for _, f := range allFindings {
		findingsByAssessment[f.AssessmentId] = append(
			findingsByAssessment[f.AssessmentId], f,
		)
	}

	for _, assessment := range assessments {
		findings := findingsByAssessment[assessment.Id]
		metric, calcErr := CalculateMetricsFromFindings(findings)
		if calcErr != nil {
			logger.Warn(nil, "Failed to calculate metrics for assessment %s: %v", assessment.Id, calcErr)
		}
		metric.Id = fmt.Sprintf("%s:%s", assessment.RepoId, assessment.AssessedAt.Format("20060102T150405"))
		metric.RepoId = assessment.RepoId
		metric.AssessedAt = assessment.AssessedAt

		dbErr := db.CreateOrUpdate(metric)
		if dbErr != nil {
			logger.Warn(dbErr, "Failed to save metric %s", metric.Id)
		}
		taskCtx.IncProgress(1)
	}

	return nil
}

func CalculateMetricsFromFindings(findings []*models.AgentReadyFinding) (*models.AgentReadyMetric, error) {
	metric := &models.AgentReadyMetric{}

	tierPass := map[int]int{1: 0, 2: 0, 3: 0, 4: 0}
	tierTotal := map[int]int{1: 0, 2: 0, 3: 0, 4: 0}
	catScoreSum := map[string]float64{}
	catCount := map[string]int{}

	for _, f := range findings {
		switch f.Status {
		case models.FindingStatusPass:
			metric.PassCount++
			tierPass[f.Tier]++
			tierTotal[f.Tier]++
		case models.FindingStatusFail:
			metric.FailCount++
			tierTotal[f.Tier]++
		default:
			metric.SkipCount++
		}

		if f.Score != nil {
			catScoreSum[f.Category] += *f.Score
			catCount[f.Category]++
		}
	}

	metric.Tier1PassRate = tierPassRate(tierPass[1], tierTotal[1])
	metric.Tier2PassRate = tierPassRate(tierPass[2], tierTotal[2])
	metric.Tier3PassRate = tierPassRate(tierPass[3], tierTotal[3])
	metric.Tier4PassRate = tierPassRate(tierPass[4], tierTotal[4])

	catAvg := map[string]float64{}
	for cat, sum := range catScoreSum {
		if catCount[cat] > 0 {
			catAvg[cat] = sum / float64(catCount[cat])
		}
	}
	catJSON, marshalErr := json.Marshal(catAvg)
	if marshalErr != nil {
		return metric, fmt.Errorf("marshaling category scores: %w", marshalErr)
	}
	metric.CategoryScores = string(catJSON)

	return metric, nil
}

func tierPassRate(pass, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(pass) / float64(total) * 100
}
