node-id = "{{ .DrainerNodeIP }}:8258"
addr = "{{ .DrainerNodeIP }}:8258"
data-dir = "data.drainer"
pd-urls = "{{ .PDUrl  }}"
initial-commit-ts = -1
detect-interval = 10
log-file = "drainer.log"
log-level = "info"

[syncer]
db-type = "oracle"
ignore-schemas = "INFORMATION_SCHEMA,PERFORMANCE_SCHEMA,mysql"
replicate-do-db = ["{{ .ReplicateDB  }}"]
sync-ddl = false
txn-batch = 100
worker-count = 20
safe-mode = false
#ignore-txn-commit-ts = [430157351441661954]

[syncer.relay]
log-dir = "/tmp/"

##schema route, oracle only
#[[syncer.table-migrate-rule]]
#[syncer.table-migrate-rule.source]
#schema = "findpt"
#[syncer.table-migrate-rule.target]
#schema = "findpt_t"

# the downstream mysql protocol database
[syncer.to]
host = "{{ .DBHost }}"
user = "{{ .DBUser }}"
password = "{{ .DBPassword }}"
port = {{ .DBPort }}
oracle-service-name = "{{ .DBName }}"
