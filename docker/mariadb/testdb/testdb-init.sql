SET FOREIGN_KEY_CHECKS=0;
SOURCE /docker-entrypoint-initdb.d/migrations/000001_schema_alignment.up.sql;
SOURCE /docker-entrypoint-initdb.d/migrations/000002_minimal_data.up.sql;
SET FOREIGN_KEY_CHECKS=1;
