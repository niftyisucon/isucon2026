DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'splunk-otel-collector') THEN
    CREATE ROLE "splunk-otel-collector" LOGIN;
  END IF;
END
$$;
GRANT pg_monitor TO "splunk-otel-collector";
GRANT CONNECT ON DATABASE isureal TO "splunk-otel-collector";
