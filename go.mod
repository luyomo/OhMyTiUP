module github.com/luyomo/OhMyTiUP

go 1.19

replace (
	github.com/appleboy/easyssh-proxy => github.com/AstroProfundis/easyssh-proxy v1.3.10-0.20210615044136-d52fc631316d
	gopkg.in/yaml.v2 => github.com/july2993/yaml v0.0.0-20200423062752-adcfa5abe2ed
)

require (
	cloud.google.com/go v0.97.0
	github.com/AstroProfundis/sysinfo v0.0.0-20210610033012-3aad056e509d
	github.com/AstroProfundis/tabby v1.1.1-color
	github.com/BurntSushi/toml v1.2.1
	github.com/ScaleFT/sshkeys v1.2.0
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38
	github.com/appleboy/easyssh-proxy v1.3.9
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/aws/aws-sdk-go-v2 v1.17.6
	github.com/aws/aws-sdk-go-v2/config v1.15.5
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.24.3
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.20.4
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.43.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.26.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.19.0
	github.com/aws/aws-sdk-go-v2/service/glue v1.43.3
	github.com/aws/aws-sdk-go-v2/service/iam v1.18.25
	github.com/aws/aws-sdk-go-v2/service/kafka v1.19.6
	github.com/aws/aws-sdk-go-v2/service/kafkaconnect v1.9.5
	github.com/aws/aws-sdk-go-v2/service/rds v1.21.1
	github.com/aws/aws-sdk-go-v2/service/redshift v1.27.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.27.11
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.9
	github.com/aws/smithy-go v1.13.5
	github.com/blevesearch/bleve v1.0.14
	github.com/cavaliercoder/grab v1.0.1-0.20201108051000-98a5bfe305ec
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/creasty/defaults v1.5.1
	github.com/docker/go-units v0.4.0
	github.com/fatih/color v1.15.0
	github.com/gibson042/canonicaljson-go v1.0.3
	github.com/go-resty/resty/v2 v2.7.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gofrs/flock v0.8.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.3
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/icholy/digest v0.1.22
	github.com/jeremywohl/flatten v1.0.1
	github.com/joomcode/errorx v1.1.0
	github.com/juju/ansiterm v1.0.0
	github.com/luyomo/tidbcloud-sdk-go-v1 v0.1.0
	github.com/mattn/go-runewidth v0.0.13
	github.com/otiai10/copy v1.9.0
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/errors v0.11.5-0.20201126102027-b0a155152ca3
	github.com/pingcap/failpoint v0.0.0-20220801062533-2eaa32854a6c
	github.com/pingcap/fn v0.0.0-20200306044125-d5540d389059
	github.com/pingcap/kvproto v0.0.0-20210622031542-706fcaf286c8
	github.com/pingcap/tidb-insight v0.3.2
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.29.0
	github.com/prometheus/prom2json v1.3.0
	github.com/r3labs/diff/v2 v2.15.1
	github.com/relex/aini v1.2.1
	github.com/sergi/go-diff v1.3.1
	github.com/shirou/gopsutil v3.21.5+incompatible
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.1
	github.com/tj/go-termd v0.0.1
	go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	go.uber.org/atomic v1.10.0
	go.uber.org/zap v1.24.0
	golang.org/x/crypto v0.7.0
	golang.org/x/mod v0.9.0
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/sys v0.6.0
	golang.org/x/term v0.6.0
	google.golang.org/api v0.57.0
	google.golang.org/genproto v0.0.0-20210930144712-2e2e1008e8a3
	google.golang.org/grpc v1.40.0
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	software.sslmate.com/src/go-pkcs12 v0.0.0-20210415151418-c5206de65a78
)

require (
	github.com/RoaringBitmap/roaring v0.4.23 // indirect
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/alecthomas/chroma v0.6.8 // indirect
	github.com/alecthomas/colour v0.0.0-20160524082231-60882d9e2721 // indirect
	github.com/alecthomas/repr v0.0.0-20180818092828-117648cd9897 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.8 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.4 // indirect
	github.com/aybabtme/rgbterm v0.0.0-20170906152045-cc83f3b3ce59 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/mmap-go v1.0.2 // indirect
	github.com/blevesearch/segment v0.9.0 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/zap/v11 v11.0.14 // indirect
	github.com/blevesearch/zap/v12 v12.0.14 // indirect
	github.com/blevesearch/zap/v13 v13.0.6 // indirect
	github.com/blevesearch/zap/v14 v14.0.5 // indirect
	github.com/blevesearch/zap/v15 v15.0.3 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/couchbase/vellum v1.0.2 // indirect
	github.com/danwakefield/fnmatch v0.0.0-20160403171240-cbb64ac3d964 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dchest/bcrypt_pbkdf v0.0.0-20150205184540-83f37f9c154a // indirect
	github.com/deepmap/oapi-codegen v1.12.4 // indirect
	github.com/dlclark/regexp2 v1.1.6 // indirect
	github.com/getkin/kin-openapi v0.116.0 // indirect
	github.com/glycerine/go-unsnap-stream v0.0.0-20181221182339-f9677308dec2 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gax-go/v2 v2.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/yaml v0.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/labstack/echo/v4 v4.10.2 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/perimeterx/marshmallow v1.1.4 // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pingcap/log v0.0.0-20200511115504-543df19646ad // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/steveyen/gtreap v0.1.0 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/tj/go-css v0.0.0-20191108133013-220a796d1705 // indirect
	github.com/tklauser/go-sysconf v0.3.6 // indirect
	github.com/tklauser/numcpus v0.2.2 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/willf/bitset v1.1.10 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	go.etcd.io/etcd/api/v3 v3.5.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)
