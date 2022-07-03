{
  "name": "mysqlSink",
  "config": {
    "topics": "clitest.public.test01",
    "input.data.format": "AVRO",
    "input.key.format": "AVRO",
    "delete.enabled": "true",
    "connector.class": "MySqlSink",
    "name": "mysqlSink",
    "kafka.auth.mode": "KAFKA_API_KEY",
    "kafka.api.key": "UKMEAUMOWZ5K3YF6",
    "kafka.api.secret": "p4o/Z3I8TizktBBuZ2e6KVl5qUqCmX8RyRLCM5DlaU2CYxLQ9Lngl2v257sN64cB",
    "connection.host": "tidb.c4604e43.fc69e292.ap-northeast-1.prod.aws.tidbcloud.com",
    "connection.port": "4000",
    "connection.user": "root",
    "connection.password": "1234Abcd",
    "db.name": "test",
    "ssl.mode": "prefer",
    "insert.mode": "UPSERT",
    "table.name.format": "test.test01",
    "table.types": "TABLE",
    "pk.mode": "record_key",
    "fields.whitelist": "col01,col02",
    "tasks.max": "1"
  }
}
