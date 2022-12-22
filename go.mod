module github.com/sarchlab/akkalab
// module model4_separate_cache_MaxNumHops

require (
	github.com/tebeka/atexit v0.3.0
	gitlab.com/akita/akita/v3 v3.0.0-alpha.16
	gitlab.com/akita/mem/v3 v3.0.0-alpha.5
	gitlab.com/akita/mgpusim/v3 v3.0.0-alpha.6
	gitlab.com/akita/noc/v3 v3.0.0-alpha.7
)

require (
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/klauspost/compress v1.14.4 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.24.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/xid v1.3.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syifan/goseth v0.1.1 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.0 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	gitlab.com/akita/dnn v0.5.3 // indirect
	go.mongodb.org/mongo-driver v1.8.4 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/tools v0.1.12 // indirect
	gonum.org/v1/gonum v0.9.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require(
	model4_separate_cache_MaxNumHops/runner v0.0.0
)
replace gitlab.com/akita/mgpusim/v3 => ../mgpusim

replace gitlab.com/akita/noc/v3 => ../noc

replace gitlab.com/akita/mem/v3 => ../mem

replace model4_separate_cache_MaxNumHops/runner => model4_separate_cache_MaxNumHops/runner

go 1.18
