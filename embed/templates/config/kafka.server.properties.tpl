broker.id={{ .BrokerID }}

# The number of threads that the server uses for receiving requests from the network and sending responses to the network
num.network.threads=3

# The number of threads that the server uses for processing requests, which may include disk I/O
num.io.threads=8

# The send buffer (SO_SNDBUF) used by the socket server
socket.send.buffer.bytes=102400

# The receive buffer (SO_RCVBUF) used by the socket server
socket.receive.buffer.bytes=102400

# The maximum size of a request that the socket server will accept (protection against OOM)
socket.request.max.bytes=104857600


############################# Log Basics #############################

# A comma separated list of directories under which to store log files
log.dirs=/var/lib/kafka
num.partitions=1
num.recovery.threads.per.data.dir=1
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1

############################# Log Flush Policy #############################
log.retention.hours=168
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000

listeners=PLAINTEXT://{{.BrokerIP}}:9092
advertised.listeners=PLAINTEXT://{{.BrokerIP}}:9092

############################# Zookeeper #############################
zookeeper.connect={{- range $idx, $data := .Zookeeper -}}{{if $idx}},{{end}}{{$data}}:2181{{- end }}

zookeeper.connection.timeout.ms=18000

##################### Confluent Metrics Reporter #######################
group.initial.rebalance.delay.ms=0
