tickTime=2000
initLimit=10
syncLimit=5
dataDir=/tmp/zookeeper/data
dataLogDir=/tmp/zookeeper/logs
clientPort=2181

{{- range $i, $data := .Zookeeper }}
server.{{$i}}={{$data}}:2888:3888
{{- end }}
