name: "one-tidb-slave"
task-mode: all
meta-schema: "dm_meta"

target-database:
  host: "private-tidb.xzgvoqakkq5.clusters.tidb-cloud.com"
  port: 4000
  user: "root"
  password: "1234Abcd"

mysql-instances:
  -
    source-id: "mysql-replica-01"
    route-rules: ["instance-1-user-rule"]
    filter-rules: ["log-filter-rule" ]

routes:
  instance-1-user-rule:
    schema-pattern: "user"
    target-schema: "test"

filters:
  log-filter-rule:
    schema-pattern: "user"
    table-pattern: "test02"
    action: Ignore
