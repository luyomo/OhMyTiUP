{
  "name": "{{ .SourceConnectorName }}",
  "config": {
      "name": "{{ .SourceConnectorName }}",
      "connector.class": "PostgresCdcSource",
      "tasks.max": "1",
      "database.hostname": "{{ .Host }}",
      "database.port": "{{ .Port }}",
      "database.user": "{{ .User }}",
      "database.password": "{{ .Password }}",
      "database.dbname" : "{{ .DBName }}",
      "database.server.name": "{{ .SourceConnectorName }}",
      "kafka.api.key": "{{ .KafkaApiKey }}",
      "kafka.api.secret": "{{ .Kfaka}} p4o/Z3I8TizktBBuZ2e6KVl5qUqCmX8RyRLCM5DlaU2CYxLQ9Lngl2v257sN64cB",
      "schema.whitelist": "public",
      "database.sslmode": "disable",
      "snapshot.mode": "initial",
      "plugin.name": "pgoutput",
      "output.data.format": "AVRO",
      "output.key.format": "AVRO"
      }
 }
