[lightning]
# Logging
level = "info"
file = "/tmp/tidb-lightning.log"

[tikv-importer]
# Configure the import mode
backend = "local"
# Sets the directory for temporarily storing the sorted key-value pairs.
# The target directory must be empty.
sorted-kv-dir = "/home/admin/tidb-lightning/sort"

[mydumper]
# Local source data directory
data-source-dir = "/home/admin/tidb-lightning/data"

# Configures the wildcard rule. By default, all tables in the mysql, sys, INFORMATION_SCHEMA, PERFORMANCE_SCHEMA, METRICS_SCHEMA, and INSPECTION_SCHEMA system databases are filtered.
# If this item is not configured, the "cannot find schema" error occurs when system tables are imported.
filter = ['*.*', '!mysql.*', '!sys.*', '!INFORMATION_SCHEMA.*', '!PERFORMANCE_SCHEMA.*', '!METRICS_SCHEMA.*', '!INSPECTION_SCHEMA.*']

[mydumper.csv]
# Separator between fields. Must be ASCII characters. It is not recommended to use the default ','. It is recommended to use '\|+\|' or other uncommon character combinations.
separator = ','
# Quoting delimiter. Empty value means no quoting.
delimiter = '"'
# Line terminator. Empty value means both "\n" (LF) and "\r\n" (CRLF) are line terminators.
terminator = ''
# Whether the CSV files contain a header.
# If `header` is true, the first line will be skipped.
header = true
# Whether the CSV contains any NULL value.
# If `not-null` is true, all columns from CSV cannot be NULL.
not-null = false
# When `not-null` is false (that is, CSV can contain NULL),
# fields equal to this value will be treated as NULL.
null = '\N'
# Whether to interpret backslash escapes inside fields.
backslash-escape = true
# If a line ends with a separator, remove it.
trim-last-separator = false

[tidb]
# Information of the target cluster
host = "{{ .TiDB }}"
port = 4000
user = "root"
password = ""
# Table schema information is fetched from TiDB via this status-port.
status-port = 10080
# The PD address of the cluster
pd-addr = "{{ .PD }}:2379"
