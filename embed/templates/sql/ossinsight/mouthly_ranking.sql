WITH stars AS (
    SELECT
        event_month,
        actor_login,
        FIRST_VALUE(repo_name) OVER (PARTITION BY repo_id ORDER BY created_at DESC) AS repo_name,
        ROW_NUMBER() OVER(PARTITION BY repo_id, actor_login) AS row_num
    FROM github_events
    USE INDEX(index_github_events_on_repo_id)
    WHERE
        type = 'WatchEvent'
        AND repo_id IN (507775, 41986369, 16563587, 60246359, 108110, 11008207, 6358188, 48833910, 114187903, 105944401, 50874442, 44781140, 166515022, 20089857, 19816070, 206444, 31006158, 99919302, 28738447, 402945349, 30753733, 372536760)
), stars_group_by_month AS (
    SELECT
        event_month,
        repo_name,
        count(DISTINCT actor_login) AS total
    FROM stars
    WHERE row_num = 1
    GROUP BY event_month, repo_name
    ORDER BY repo_name, event_month
), stars_group_by_repo AS (
    SELECT
        repo_name,
        count(DISTINCT actor_login) AS total
    FROM stars
    GROUP BY repo_name
    ORDER BY repo_name
), stars_current_month AS (
    SELECT
        event_month,
        repo_name,
        total,
        ROW_NUMBER() OVER(PARTITION BY event_month ORDER BY total DESC) AS `rank`
    FROM stars_group_by_month sgn
    WHERE event_month = DATE_FORMAT(date_sub(now(), interval DAYOFMONTH(now()) day), '%Y-%m-01')
), stars_last_month AS (
    SELECT
        event_month,
        repo_name,
        total,
        ROW_NUMBER() OVER(PARTITION BY event_month ORDER BY total DESC) AS `rank`
    FROM stars_group_by_month sgn
    WHERE event_month = DATE_FORMAT(date_sub(date_sub(now(), interval DAYOFMONTH(now()) day), interval 1 month), '%Y-%m-01')
)
SELECT
    scm.repo_name,
    DATE_FORMAT(date_sub(now(), interval DAYOFMONTH(now()) day), '%Y-%m') AS current_month,
    DATE_FORMAT(date_sub(date_sub(now(), interval DAYOFMONTH(now()) day), interval 1 month), '%Y-%m') AS last_month,
    -- Stars
    scm.total AS current_month_total,
    scm.`rank` AS current_month_rank,
    slm.total AS last_month_total,
    slm.`rank` AS last_month_rank,
    ((scm.total - slm.total) / slm.total) * 100 AS total_mom,
    (scm.`rank` - slm.`rank`) AS rank_mom,
    sgr.total AS total
FROM stars_group_by_repo sgr 
JOIN stars_current_month scm ON sgr.repo_name = scm.repo_name
JOIN stars_last_month slm ON scm.repo_name = slm.repo_name
ORDER BY current_month_rank;
