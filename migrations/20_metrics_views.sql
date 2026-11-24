CREATE OR REPLACE VIEW v_pin_engagement AS
SELECT
    p.pin_id,
    p.creator_id,
    p.name,
    COALESCE(l.likes,    0) AS likes_count,
    COALESCE(c.comments, 0) AS comments_count,
    COALESCE(s.saves,    0) AS saves_count
FROM pin AS p
LEFT JOIN (SELECT pin_id, count(*) AS likes
             FROM pin_like GROUP BY pin_id) AS l ON l.pin_id = p.pin_id
LEFT JOIN (SELECT pin_id, count(*) AS comments
             FROM commentary WHERE deleted_at IS NULL GROUP BY pin_id) AS c ON c.pin_id = p.pin_id
LEFT JOIN (SELECT pin_id, count(*) AS saves
             FROM pin_desk_relation GROUP BY pin_id) AS s ON s.pin_id = p.pin_id
WHERE p.deleted_at IS NULL;

CREATE OR REPLACE VIEW v_metrics_overview AS
SELECT
    (SELECT count(DISTINCT user_id) FROM user_session
      WHERE session_end >= now() - interval '1 day')              AS active_users_day,
    (SELECT count(DISTINCT user_id) FROM user_session
      WHERE session_end >= now() - interval '7 days')             AS active_users_week,
    COALESCE((SELECT extract(epoch FROM avg(session_end - session_start))
                FROM user_session
               WHERE session_end >= now() - interval '7 days'), 0)::double precision
                                                                  AS avg_session_seconds_week,
    (SELECT count(*) FROM pin
      WHERE created_at >= now() - interval '1 day'
        AND deleted_at IS NULL)                                   AS new_pins_day,
    (SELECT count(*) FROM pin_like
      WHERE created_at >= now() - interval '1 day')               AS new_likes_day,
    (SELECT count(*) FROM commentary
      WHERE created_at >= now() - interval '1 day'
        AND deleted_at IS NULL)                                   AS new_comments_day,
    (SELECT count(*) FROM pin_desk_relation
      WHERE created_at >= now() - interval '1 day')               AS pin_saves_day,
    (SELECT count(*) FROM subscription
      WHERE created_at >= now() - interval '1 day')               AS new_subscriptions_day,
    (SELECT count(*) FROM chat_message
      WHERE created_at >= now() - interval '1 day'
        AND deleted_at IS NULL)                                   AS chat_messages_day,
    (SELECT count(*) FROM feedback
      WHERE created_at >= now() - interval '1 day')               AS new_feedback_day,
    (SELECT count(*) FROM feedback
      WHERE created_at >= now() - interval '1 day'
        AND is_complaint)                                         AS new_complaints_day,
    (SELECT count(*) FROM "user" WHERE deleted_at IS NULL)        AS users_total,
    (SELECT count(*) FROM pin WHERE deleted_at IS NULL)           AS pins_total,
    COALESCE((SELECT count(*) FROM pin_like)::double precision
             / NULLIF((SELECT count(*) FROM pin WHERE deleted_at IS NULL), 0), 0) AS avg_likes_per_pin,
    COALESCE((SELECT count(*) FROM commentary WHERE deleted_at IS NULL)::double precision
             / NULLIF((SELECT count(*) FROM pin WHERE deleted_at IS NULL), 0), 0) AS avg_comments_per_pin;

COMMENT ON VIEW v_metrics_overview IS 'Продуктовые метрики одной строкой (скользящие окна 24ч/7д)';
COMMENT ON VIEW v_pin_engagement   IS 'Лайки, комментарии и сохранения на каждый пин';

ALTER VIEW v_metrics_overview OWNER TO migrator;
ALTER VIEW v_pin_engagement   OWNER TO migrator;

GRANT SELECT ON v_metrics_overview, v_pin_engagement TO admin_ro;
