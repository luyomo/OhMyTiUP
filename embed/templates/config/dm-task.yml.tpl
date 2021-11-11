name: "test"
task-mode: "all"

ignore-checking-items: ["replication_privilege", "dump_privilege"]
target-database:
  host: "172.83.1.183"
  port: 4000
  user: "root"
  password: ""

mysql-instances:
-
  source-id: "mysql-replica-01"
  block-allow-list: "global"  # Use black-white-list if the DM's version <= v2.0.0-beta.2.
  mydumper-config-name: "global"

block-allow-list:                     # Use black-white-list if the DM's version <= v2.0.0-beta.2.
  global:
    do-tables:                        # The allow list of upstream tables to be migrated.
    - db-name: "dm_test"              # The database name of the table to be migrated.
      tbl-name: "test02"          # The name of the table to be migrated.

# The global configuration of the dump unit. Each instance can quote it by the configuration item name.
mydumpers:
  global:
    extra-args: " --no-locks"
