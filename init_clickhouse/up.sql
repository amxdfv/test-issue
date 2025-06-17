CREATE DATABASE test_issue;
USE test_issue;

CREATE TABLE IF NOT EXISTS test_issue.goods (
id integer,
project_id integer,
name text,
description text,
priority integer ,
removed bool DEFAULT false,
event_time timestamp DEFAULT now()
)
ENGINE = NATS
   SETTINGS nats_url = 'nats.local:4222',
             nats_subjects = 'test_issue',
             nats_format = 'JSONEachRow',
             date_time_input_format = 'best_effort'
;

CREATE MATERIALIZED VIEW summary_view
ENGINE = SummingMergeTree()
ORDER BY (event_time)
AS
SELECT
    *
FROM test_issue.goods;