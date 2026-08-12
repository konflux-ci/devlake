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

package pr

import (
	"context"
	"database/sql"
	"fmt"
)

// withSortBuffer executes fn on a dedicated connection after bumping the
// session sort_buffer_size to sizeMB megabytes.  Use for queries that do a
// large GROUP BY or ORDER BY on wide rows.
func withSortBuffer(ctx context.Context, db *sql.DB, sizeMB int, fn func(*sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("SET SESSION sort_buffer_size = %d", sizeMB*1024*1024)); err != nil {
		return fmt.Errorf("set sort_buffer_size: %w", err)
	}
	return fn(conn)
}

// BasePRs returns one PRRow per PR that was created or closed within [p.From, p.To].
// Bot filtering (basic SQL patterns) is applied; the transform layer applies
// the more nuanced github-actions[bot] content-check on top.
// Used by: cycle-time, flow, productivity, z-score, scatter.
func BasePRs(ctx context.Context, db *sql.DB, p Params) ([]PRRow, error) {
	args := []interface{}{p.BlueprintID}
	args = append(args, repoArgs(p.Repos)...)
	wlSQL := whitelistSQL(p, "pr", &args)
	args = append(args, p.From, p.To, p.From, p.To)

	q := fmt.Sprintf( //nolint:gosec // G201: %s slots only receive placeholders()/whitelistSQL() output, never raw user input
		`
SELECT
  pr.id,
  COALESCE(pr.pull_request_key, ''),
  COALESCE(pr.url, ''),
  pr.status,
  pr.created_date,
  pr.merged_date,
  pr.closed_date,
  COALESCE(pr.author_name, ''),
  COALESCE(pr.title, ''),
  COALESCE(pr.description, ''),
  pr.is_draft,
  COALESCE(pr.additions, 0),
  COALESCE(pr.deletions, 0),
  COALESCE(GROUP_CONCAT(DISTINCT prl.label_name SEPARATOR ','), '') AS prow_labels
FROM pull_requests pr
JOIN project_mapping pm ON pm.`+"`table`"+` = 'repos' AND pm.row_id = pr.base_repo_id
JOIN _devlake_blueprints bp ON bp.project_name = pm.project_name
JOIN repos r ON r.id = pr.base_repo_id
LEFT JOIN pull_request_labels prl
  ON prl.pull_request_id = pr.id AND LOWER(prl.label_name) IN ('approved', 'lgtm')
WHERE FIND_IN_SET(bp.id, ?)
  AND r.name IN %s
  %s
  AND pr.author_name IS NOT NULL AND pr.author_name != ''
  AND pr.created_date IS NOT NULL
  AND (
    (UNIX_TIMESTAMP(pr.closed_date)  BETWEEN ? AND ?)
    OR (UNIX_TIMESTAMP(pr.created_date) BETWEEN ? AND ?)
  )
GROUP BY
  pr.id, pr.pull_request_key, pr.url, pr.status,
  pr.created_date, pr.merged_date, pr.closed_date,
  pr.author_name, pr.title, pr.description, pr.is_draft,
  pr.additions, pr.deletions
`, placeholders(len(p.Repos)), wlSQL)

	var out []PRRow
	err := withSortBuffer(ctx, db, 8, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("BasePRs query: %w", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var r PRRow
			if err := rows.Scan(
				&r.ID, &r.Key, &r.URL, &r.Status,
				&r.CreatedDate, &r.MergedDate, &r.ClosedDate,
				&r.AuthorName, &r.Title, &r.Description,
				&r.IsDraft, &r.Additions, &r.Deletions,
				&r.ProwLabels,
			); err != nil {
				return fmt.Errorf("BasePRs scan: %w", err)
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

// Reviews returns review-comments for all PRs scoped by the same repo/blueprint
// filter.  Only non-empty author accounts are included (the transform layer
// applies the bot check on AuthorName).
// Used by: flow.
func Reviews(ctx context.Context, db *sql.DB, p Params) ([]ReviewRow, error) {
	args := []interface{}{p.BlueprintID}
	args = append(args, repoArgs(p.Repos)...)
	args = append(args, p.From, p.To, p.From, p.To)

	q := fmt.Sprintf( //nolint:gosec // G201: %s slots only receive placeholders()/whitelistSQL() output, never raw user input
		`
SELECT
  prc.pull_request_id,
  prc.created_date,
  COALESCE(prc.body, ''),
  COALESCE(prc.status, ''),
  COALESCE(a.user_name, '')
FROM pull_request_comments prc
JOIN accounts a ON prc.account_id = a.id
JOIN pull_requests pr ON prc.pull_request_id = pr.id
JOIN project_mapping pm ON pm.`+"`table`"+` = 'repos' AND pm.row_id = pr.base_repo_id
JOIN _devlake_blueprints bp ON bp.project_name = pm.project_name
JOIN repos r ON r.id = pr.base_repo_id
WHERE FIND_IN_SET(bp.id, ?)
  AND r.name IN %s
  AND a.user_name IS NOT NULL AND a.user_name != ''
  AND (
    (UNIX_TIMESTAMP(pr.closed_date)  BETWEEN ? AND ?)
    OR (UNIX_TIMESTAMP(pr.created_date) BETWEEN ? AND ?)
  )
ORDER BY prc.pull_request_id, prc.created_date
`, placeholders(len(p.Repos)))

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("Reviews query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ReviewRow
	for rows.Next() {
		var r ReviewRow
		if err := rows.Scan(
			&r.PullRequestID, &r.CreatedDate, &r.Body, &r.Status, &r.AuthorName,
		); err != nil {
			return nil, fmt.Errorf("Reviews scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// KeyMetricsStats returns pre-aggregated counts and medians for the key-metrics
// widget. The SQL does all heavy lifting (window-function median) so the
// transform layer only needs to format the output.
func KeyMetricsStats(ctx context.Context, db *sql.DB, p Params) (KeyMetricsStatsRow, error) {
	args := []interface{}{p.BlueprintID}
	args = append(args, repoArgs(p.Repos)...)
	wlSQL := whitelistSQL(p, "pr", &args)
	args = append(args, p.From, p.To)
	// opened_prs subquery: same blueprint + repo + whitelist args
	args = append(args, p.BlueprintID)
	args = append(args, repoArgs(p.Repos)...)
	wl2SQL := whitelistSQL(p, "pr2", &args)
	args = append(args, p.From, p.To)

	q := fmt.Sprintf( //nolint:gosec // G201: %s slots only receive placeholders()/whitelistSQL() output, never raw user input
		`
WITH closed_prs AS (
  SELECT
    pr.id,
    pr.pull_request_key,
    pr.url,
    TIMESTAMPDIFF(SECOND, pr.created_date, COALESCE(pr.merged_date, pr.closed_date)) / 3600.0 AS cycle_time_hours
  FROM pull_requests pr
  JOIN project_mapping pm ON pm.`+"`table`"+` = 'repos' AND pm.row_id = pr.base_repo_id
  JOIN _devlake_blueprints bp ON bp.project_name = pm.project_name
  JOIN repos r ON r.id = pr.base_repo_id
  WHERE FIND_IN_SET(bp.id, ?)
    AND r.name IN %s
    %s
    AND pr.status != 'OPEN'
    AND NOT pr.is_draft
    AND pr.author_name IS NOT NULL AND pr.author_name != ''
    AND pr.created_date IS NOT NULL
    AND UNIX_TIMESTAMP(COALESCE(pr.merged_date, pr.closed_date)) BETWEEN ? AND ?
    AND TIMESTAMPDIFF(SECOND, pr.created_date, COALESCE(pr.merged_date, pr.closed_date)) >= 0
  GROUP BY pr.id, pr.pull_request_key, pr.url, pr.created_date, pr.merged_date, pr.closed_date
),
ranked AS (
  SELECT cycle_time_hours,
    ROW_NUMBER() OVER (ORDER BY cycle_time_hours) AS rn,
    COUNT(*) OVER () AS cnt
  FROM closed_prs
),
median AS (
  SELECT COALESCE(AVG(cycle_time_hours), 0) AS val
  FROM ranked
  WHERE rn IN (FLOOR((cnt+1)/2), CEIL((cnt+1)/2))
)
SELECT
  (SELECT COUNT(*) FROM closed_prs)                                                                 AS total_closed_prs,
  (SELECT val FROM median)                                                                           AS median_cycle_time,
  (SELECT COUNT(DISTINCT pr2.id)
   FROM pull_requests pr2
   JOIN project_mapping pm2 ON pm2.`+"`table`"+` = 'repos' AND pm2.row_id = pr2.base_repo_id
   JOIN _devlake_blueprints bp2 ON bp2.project_name = pm2.project_name
   JOIN repos r2 ON r2.id = pr2.base_repo_id
   WHERE FIND_IN_SET(bp2.id, ?)
     AND r2.name IN %s
     %s
     AND NOT pr2.is_draft
     AND pr2.author_name IS NOT NULL AND pr2.author_name != ''
     AND UNIX_TIMESTAMP(pr2.created_date) BETWEEN ? AND ?)                                          AS opened_prs,
  (SELECT COUNT(*) FROM closed_prs WHERE cycle_time_hours > 3 * (SELECT val FROM median))           AS outlier_prs,
  0                                                                                                  AS median_interaction_time
`,
		placeholders(len(p.Repos)), wlSQL,
		placeholders(len(p.Repos)), wl2SQL,
	)

	var r KeyMetricsStatsRow
	err := db.QueryRowContext(ctx, q, args...).Scan(
		&r.TotalClosedPRs,
		&r.MedianCycleTime,
		&r.OpenedPRs,
		&r.OutlierPRs,
		&r.MedianInteractionTime,
	)
	if err != nil {
		return r, fmt.Errorf("KeyMetricsStats: %w", err)
	}
	return r, nil
}

// KeyMetricsOutliers returns PRs whose cycle time exceeds 3× the median for
// the given period.  Used to populate the drill-down list in the key-metrics widget.
func KeyMetricsOutliers(ctx context.Context, db *sql.DB, p Params) ([]OutlierPRRow, error) {
	args := []interface{}{p.BlueprintID}
	args = append(args, repoArgs(p.Repos)...)
	wlSQL := whitelistSQL(p, "pr", &args)
	args = append(args, p.From, p.To)

	q := fmt.Sprintf( //nolint:gosec // G201: %s slots only receive placeholders()/whitelistSQL() output, never raw user input
		`
WITH closed_prs AS (
  SELECT
    pr.pull_request_key,
    pr.url,
    TIMESTAMPDIFF(SECOND, pr.created_date, COALESCE(pr.merged_date, pr.closed_date)) / 3600.0 AS cycle_time_hours
  FROM pull_requests pr
  JOIN project_mapping pm ON pm.`+"`table`"+` = 'repos' AND pm.row_id = pr.base_repo_id
  JOIN _devlake_blueprints bp ON bp.project_name = pm.project_name
  JOIN repos r ON r.id = pr.base_repo_id
  WHERE FIND_IN_SET(bp.id, ?)
    AND r.name IN %s
    %s
    AND pr.status != 'OPEN'
    AND NOT pr.is_draft
    AND pr.author_name IS NOT NULL AND pr.author_name != ''
    AND pr.created_date IS NOT NULL
    AND UNIX_TIMESTAMP(COALESCE(pr.merged_date, pr.closed_date)) BETWEEN ? AND ?
    AND TIMESTAMPDIFF(SECOND, pr.created_date, COALESCE(pr.merged_date, pr.closed_date)) >= 0
  GROUP BY pr.id, pr.pull_request_key, pr.url, pr.created_date, pr.merged_date, pr.closed_date
),
ranked AS (
  SELECT cycle_time_hours,
    ROW_NUMBER() OVER (ORDER BY cycle_time_hours) AS rn,
    COUNT(*) OVER () AS cnt
  FROM closed_prs
),
median AS (
  SELECT COALESCE(AVG(cycle_time_hours), 0) AS val
  FROM ranked WHERE rn IN (FLOOR((cnt+1)/2), CEIL((cnt+1)/2))
)
SELECT pull_request_key, url, cycle_time_hours
FROM closed_prs
WHERE cycle_time_hours > 3 * (SELECT val FROM median)
ORDER BY cycle_time_hours DESC
`, placeholders(len(p.Repos)), wlSQL)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("KeyMetricsOutliers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []OutlierPRRow
	for rows.Next() {
		var r OutlierPRRow
		if err := rows.Scan(&r.PRNumber, &r.PRURL, &r.CycleTimeHours); err != nil {
			return nil, fmt.Errorf("KeyMetricsOutliers scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StagesPRs returns one StagePRRow per MERGED PR that has a complete
// pickup→review→integration chain.  Averages are computed in the transform
// layer so the same data can also drive per-PR drill-down lists.
//
// Stage definitions (matching node-metric_pr_stages_alternate2.js):
//   - Pickup:      PR created → first non-author review comment
//   - Review:      first review → last approval (Prow /approve or native)
//   - Integration: last approval → merge
func StagesPRs(ctx context.Context, db *sql.DB, p Params) ([]StagePRRow, error) {
	args := []interface{}{p.BlueprintID}
	args = append(args, repoArgs(p.Repos)...)
	wlSQL := whitelistSQL(p, "pr", &args)
	args = append(args, p.From, p.To, p.From, p.To)

	q := fmt.Sprintf( //nolint:gosec // G201: %s slots only receive placeholders()/whitelistSQL() output, never raw user input
		`
WITH base_prs AS (
  SELECT pr.id, pr.status, pr.created_date, pr.merged_date, pr.author_name
  FROM pull_requests pr
  JOIN project_mapping pm ON pm.`+"`table`"+` = 'repos' AND pm.row_id = pr.base_repo_id
  JOIN _devlake_blueprints bp ON bp.project_name = pm.project_name
  JOIN repos r ON r.id = pr.base_repo_id
  WHERE FIND_IN_SET(bp.id, ?)
    AND r.name IN %s
    %s
    AND pr.status = 'MERGED'
    AND pr.merged_date IS NOT NULL
    AND pr.author_name IS NOT NULL AND pr.author_name != ''
    AND NOT pr.is_draft
    AND pr.created_date IS NOT NULL
    AND (
      (UNIX_TIMESTAMP(pr.merged_date)  BETWEEN ? AND ?)
      OR (UNIX_TIMESTAMP(pr.created_date) BETWEEN ? AND ?)
    )
  GROUP BY pr.id, pr.status, pr.created_date, pr.merged_date, pr.author_name
),
non_bot_comments AS (
  SELECT prc.pull_request_id, prc.created_date, prc.status, prc.body
  FROM pull_request_comments prc
  JOIN accounts a ON prc.account_id = a.id
  JOIN base_prs bp ON prc.pull_request_id = bp.id
  WHERE a.user_name != bp.author_name
    AND a.user_name IS NOT NULL AND a.user_name != ''
    AND NOT (
      LOWER(a.user_name) LIKE '%%[bot]%%' OR LOWER(a.user_name) LIKE '%%copilot%%'
      OR LOWER(a.user_name) LIKE '%%github-actions%%' OR LOWER(a.user_name) LIKE '%%github actions%%'
      OR LOWER(a.user_name) LIKE '%%dependabot%%'
      OR LOWER(a.user_name) LIKE '%%-bot' OR LOWER(a.user_name) LIKE '%%-robot'
    )
),
first_reviews AS (
  SELECT pull_request_id, MIN(created_date) AS first_review_date
  FROM non_bot_comments
  GROUP BY pull_request_id
),
prow_labels AS (
  SELECT pull_request_id,
    MAX(CASE WHEN LOWER(label_name) = 'approved' THEN 1 ELSE 0 END) AS has_approved,
    MAX(CASE WHEN LOWER(label_name) = 'lgtm'     THEN 1 ELSE 0 END) AS has_lgtm
  FROM pull_request_labels
  WHERE pull_request_id IN (SELECT id FROM base_prs)
    AND LOWER(label_name) IN ('approved', 'lgtm')
  GROUP BY pull_request_id
),
last_approve AS (
  SELECT pull_request_id, MAX(created_date) AS approve_date
  FROM non_bot_comments
  WHERE body REGEXP '/approve([^[:alnum:]_]|$)'
    AND body NOT LIKE '%%[APPROVALNOTIFIER]%%'
  GROUP BY pull_request_id
),
last_lgtm AS (
  SELECT pull_request_id, MAX(created_date) AS lgtm_date
  FROM non_bot_comments
  WHERE body REGEXP '/lgtm([^[:alnum:]_]|$)'
  GROUP BY pull_request_id
),
native_last_approval AS (
  SELECT pull_request_id, MAX(created_date) AS approval_date
  FROM non_bot_comments
  WHERE status = 'APPROVED'
  GROUP BY pull_request_id
),
last_approval AS (
  SELECT bp.id,
    CASE
      WHEN COALESCE(pl.has_approved, 0) = 1 AND COALESCE(pl.has_lgtm, 0) = 1 THEN
        COALESCE(
          CASE
            WHEN la.approve_date IS NOT NULL AND ll.lgtm_date IS NOT NULL
              THEN GREATEST(la.approve_date, ll.lgtm_date)
            ELSE COALESCE(la.approve_date, ll.lgtm_date)
          END,
          na.approval_date
        )
      WHEN COALESCE(pl.has_approved, 0) = 1 THEN COALESCE(la.approve_date, na.approval_date)
      WHEN COALESCE(pl.has_lgtm,     0) = 1 THEN COALESCE(ll.lgtm_date,   na.approval_date)
      ELSE na.approval_date
    END AS approval_date
  FROM base_prs bp
  LEFT JOIN prow_labels pl ON bp.id = pl.pull_request_id
  LEFT JOIN last_approve la ON bp.id = la.pull_request_id
  LEFT JOIN last_lgtm    ll ON bp.id = ll.pull_request_id
  LEFT JOIN native_last_approval na ON bp.id = na.pull_request_id
)
SELECT
  COALESCE(pr.pull_request_key, ''),
  COALESCE(pr.url, ''),
  TIMESTAMPDIFF(SECOND, bp.created_date, fr.first_review_date) / 3600.0,
  TIMESTAMPDIFF(SECOND, fr.first_review_date, ad.approval_date) / 3600.0,
  TIMESTAMPDIFF(SECOND, ad.approval_date, bp.merged_date) / 3600.0
FROM base_prs bp
JOIN pull_requests pr ON pr.id = bp.id
JOIN first_reviews fr ON bp.id = fr.pull_request_id
JOIN last_approval ad ON bp.id = ad.id
WHERE ad.approval_date IS NOT NULL
  AND TIMESTAMPDIFF(SECOND, bp.created_date, fr.first_review_date) >= 0
  AND TIMESTAMPDIFF(SECOND, fr.first_review_date, ad.approval_date) >= 0
  AND TIMESTAMPDIFF(SECOND, ad.approval_date, bp.merged_date) >= 0
`, placeholders(len(p.Repos)), wlSQL)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("StagesPRs query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []StagePRRow
	for rows.Next() {
		var r StagePRRow
		if err := rows.Scan(&r.Key, &r.URL, &r.PickupHours, &r.ReviewHours, &r.IntegrationHours); err != nil {
			return nil, fmt.Errorf("StagesPRs scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
