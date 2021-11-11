case-sensitive = true

enable-old-value = true

[filter]
ignore-txn-start-ts = [1, 2]

rules = ['cdc_test.test02']

[mounter]
# mounter thread counts, which is used to decode the TiKV output data.
worker-num = 16

[sink]
dispatchers = [
    {matcher = ['test1.*', 'test2.*'], dispatcher = "ts"},
    {matcher = ['test3.*', 'test4.*'], dispatcher = "rowid"},
]
protocol = "default"

[cyclic-replication]
enable = false
replica-id = 1
filter-replica-ids = [2,3]
sync-ddl = true
