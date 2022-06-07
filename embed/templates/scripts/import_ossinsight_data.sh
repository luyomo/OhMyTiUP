#!/bin/bash

TABLES=(ar_internal_metadata blacklist_repos blacklist_users cn_orgs cn_repos collection_items collections css_framework_repos db_repos gh import_logs js_framework_repos new_github_events nocode_repos osdb_repos programming_language_repos schema_migrations static_site_generator_repos users)

for file in ${TABLES[@]}
do
  wget -P /tmp/ https://ossinsight-data.s3.amazonaws.com/parquet/$file.part_00000
  /opt/scripts/run_mysql_query ossinsight "LOAD DATA LOCAL INFILE '/tmp/$file.part_00000' INTO TABLE $file"
  rm /tmp/$file.part_00000
done

