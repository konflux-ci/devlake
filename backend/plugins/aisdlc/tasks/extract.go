/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package tasks

import (
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/aisdlc/models"
	"github.com/apache/incubator-devlake/plugins/aisdlc/tasks/parse"
)

var ExtractObservationsMeta = plugin.SubTaskMeta{
	Name:             "extractObservations",
	EntryPoint:       ExtractObservations,
	EnabledByDefault: true,
	Description:      "Convert station artifacts into observations",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&CollectFilesMeta},
}

// ExtractObservations parses stored blobs into observations.
// Reading the same blob again updates collected_at and does not add a row.
func ExtractObservations(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AisdlcTaskData)
	cfg := data.Options.ScopeConfig
	projectPath := data.Options.FullName

	var raws []models.AisdlcRawFile
	if err := db.All(&raws, dal.Where("connection_id = ? AND project_path = ?", data.Connection.ID, projectPath)); err != nil {
		return err
	}
	prefix := strings.Trim(cfg.PathPrefix, "/")
	files := make([]parse.File, 0, len(raws))
	for _, raw := range raws {
		if prefix != "" && raw.FilePath != prefix && !strings.HasPrefix(raw.FilePath, prefix+"/") {
			continue
		}
		files = append(files, parse.File{
			Path:      raw.FilePath,
			BlobID:    raw.BlobId,
			CommitSHA: raw.CommitSha,
			Body:      raw.Body,
		})
	}
	webBase := ""
	if glConn, glErr := LoadGitlabConnection(db, data.Connection.GitlabConnectionId); glErr == nil {
		webBase = WebBase(glConn.Endpoint)
	}
	set := parse.Parse(parse.Config{
		TenantID:    cfg.TenantId,
		SourceID:    cfg.SourceId,
		StationID:   cfg.StationId,
		DisplayName: cfg.DisplayName,
		Parser:      cfg.Parser,
		ProjectPath: projectPath,
		WebBase:     webBase,
	}, files, time.Now().UTC())
	for _, message := range set.FileErrors {
		logger.Warn(nil, "parse %s", message)
	}
	return saveDraft(db, data.Connection.ID, projectPath, cfg, set)
}

func saveDraft(db dal.Dal, connectionID uint64, projectPath string, cfg *models.AisdlcScopeConfig, set parse.DraftSet) errors.Error {
	for _, exec := range set.Executions {
		row := &models.AisdlcExecution{
			Id:               exec.ID,
			ConnectionId:     connectionID,
			SourceId:         exec.SourceID,
			StationId:        exec.StationID,
			SourceRunId:      exec.SourceRunID,
			RunIdType:        exec.RunIDType,
			PipelineIdStatus: exec.PipelineIDStatus,
			ProjectPath:      projectPath,
			BatchPath:        exec.BatchPath,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	for _, item := range set.WorkItems {
		existing := &models.AisdlcWorkItem{}
		err := db.First(existing, dal.Where("id = ? AND connection_id = ?", item.ID, connectionID))
		matchStatus := item.MatchStatus
		domainID := item.DomainIssueID
		if err == nil && existing.MatchStatus == parse.MatchUnique {
			matchStatus = existing.MatchStatus
			domainID = existing.DomainIssueId
		}
		row := &models.AisdlcWorkItem{
			Id:             item.ID,
			ConnectionId:   connectionID,
			SourceSystem:   item.SourceSystem,
			SourceInstance: item.SourceInstance,
			EntityKind:     item.EntityKind,
			ItemKey:        item.ItemKey,
			IdStatus:       item.IdStatus,
			DomainIssueId:  domainID,
			MatchStatus:    matchStatus,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	for _, obs := range set.Observations {
		row := &models.AisdlcObservation{
			Id:              obs.ID,
			ConnectionId:    connectionID,
			SchemaVersion:   parse.SchemaVersion,
			TenantId:        cfg.TenantId,
			SourceId:        cfg.SourceId,
			StationId:       cfg.StationId,
			ObservationKind: obs.Kind,
			WorkItemId:      obs.WorkItemID,
			ExecutionId:     obs.ExecutionID,
			AttemptStatus:   obs.AttemptStatus,
			RawResult:       obs.RawResult,
			AssessmentTotal: obs.AssessmentTotal,
			AssessmentScale: obs.AssessmentScale,
			RawVerdict:      obs.RawVerdict,
			ProjectPath:     projectPath,
			FilePath:        obs.Path,
			BlobId:          obs.BlobID,
			CommitSha:       obs.CommitSHA,
			FieldSelector:   obs.FieldSelector,
			AdapterId:       obs.AdapterID,
			AdapterVersion:  parse.AdapterVersion,
			MappingVersion:  parse.MappingVersion,
			Completeness:    obs.Completeness,
			Extensions:      obs.Extensions,
			CollectedAt:     obs.CollectedAt,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
		if err := replaceChildren(db, connectionID, obs); err != nil {
			return err
		}
	}
	return nil
}

func replaceChildren(db dal.Dal, connectionID uint64, obs parse.Observation) errors.Error {
	where := dal.Where("connection_id = ? AND observation_id = ?", connectionID, obs.ID)
	if err := db.Delete(&models.AisdlcEvidence{}, where); err != nil {
		return err
	}
	if err := db.Delete(&models.AisdlcItemLink{}, where); err != nil {
		return err
	}
	if err := db.Delete(&models.AisdlcUnknown{}, where); err != nil {
		return err
	}
	if err := db.Delete(&models.AisdlcTime{}, where); err != nil {
		return err
	}
	for _, ev := range obs.Evidence {
		row := &models.AisdlcEvidence{
			Id:            ev.ID,
			ConnectionId:  connectionID,
			ObservationId: obs.ID,
			SourceId:      ev.SourceID,
			URI:           ev.URI,
			Revision:      ev.Revision,
			FieldSelector: ev.FieldSelector,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	for _, link := range obs.Links {
		row := &models.AisdlcItemLink{
			Id:             link.ID,
			ConnectionId:   connectionID,
			ObservationId:  obs.ID,
			Role:           link.Role,
			Relation:       link.Relation,
			WorkItemId:     link.WorkItemID,
			EvidenceStatus: link.EvidenceStatus,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	for _, unknown := range obs.Unknowns {
		row := &models.AisdlcUnknown{
			Id:             unknown.ID,
			ConnectionId:   connectionID,
			ObservationId:  obs.ID,
			Field:          unknown.Field,
			Reason:         unknown.Reason,
			EvidenceNeeded: unknown.EvidenceNeeded,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	for _, stamp := range obs.Times {
		var normalized *string
		if stamp.Normalized != "" {
			value := stamp.Normalized
			normalized = &value
		}
		row := &models.AisdlcTime{
			Id:             parse.ID(obs.ID, "time", stamp.Type, stamp.Raw),
			ConnectionId:   connectionID,
			ObservationId:  obs.ID,
			TimeType:       stamp.Type,
			RawValue:       stamp.Raw,
			Normalized:     normalized,
			Precision:      stamp.Precision,
			TimezoneStatus: stamp.TimezoneStatus,
			AppliesTo:      stamp.AppliesTo,
		}
		if err := db.CreateOrUpdate(row); err != nil {
			return err
		}
	}
	return nil
}
