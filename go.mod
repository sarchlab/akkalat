module github.com/sarchlab/akkalab

require (
	github.com/tebeka/atexit v0.3.0
	gitlab.com/akita/akita/v3 v3.0.0-alpha.19
	gitlab.com/akita/mem/v3 v3.0.0-alpha.8
	gitlab.com/akita/mgpusim/v3 v3.0.0-alpha.6
	gitlab.com/akita/noc/v3 v3.0.0-alpha.8
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/montanaflynn/stats v0.7.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syifan/goseth v0.1.1 // indirect
	github.com/tidwall/pretty v1.0.2 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	gitlab.com/akita/dnn v0.5.3 // indirect
	go.mongodb.org/mongo-driver v1.11.2 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	gonum.org/v1/gonum v0.9.0 // indirect
)

replace gitlab.com/akita/mgpusim/v3 => ../mgpusim

// replace gitlab.com/akita/noc/v3 => ../noc

replace gitlab.com/akita/mem/v3 => ../mem

replace gitlab.com/akita/akita/v3 => ../akita

go 1.18
