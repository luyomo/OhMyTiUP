{
  "name": "{{ .SINKName }}",
  "config": {
    "connector.class": "io.confluent.connect.aws.redshift.RedshiftSinkConnector",
    "tasks.max": "1",
    "confluent.topic.bootstrap.servers": "{{ .KafkaBroker }}",
    "topics": "{{ .TopicName }}",
    "aws.redshift.domain": "{{ .RedshiftHost }}",
    "aws.redshift.port": "{{ .RedshiftPort }}",
    "aws.redshift.database": "{{ .RedshiftDBName }}",
    "aws.redshift.user": "{{ .RedshiftUser }}",
    "aws.redshift.password": "{{ .RedshiftPassword }}",
    "table.name.format": "{{ .TableName }}",
    "insert.mode": "insert",
    "delete.enabled": "true",
    "pk.mode": "record_key",
    "auto.create": "true"
  }
}
