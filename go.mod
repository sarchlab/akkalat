module github.com/sarchlab/akkalat

require (
	github.com/sarchlab/akita/v3 v3.0.0-alpha.28.0.20230616154900-5e2fda40a106
	github.com/sarchlab/mgpusim/v3 v3.0.0-20230622042936-16aa3c53211e
	github.com/tebeka/atexit v0.3.0
)

require (
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-sql-driver/mysql v1.7.1 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.16 // indirect
	github.com/rs/xid v1.5.0 // indirect
	// github.com/sarchlab/mgpusim/v3 v3.0.0-20230620043528-e67cf84c1c45 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syifan/goseth v0.1.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	gitlab.com/akita/dnn v0.5.4 // indirect
	golang.org/x/sys v0.9.0 // indirect
	gonum.org/v1/gonum v0.13.0 // indirect
)

replace gitlub.com/sarchlab/akita/mgpusim/v3 => ../mgpusim

// replace gitlub.com/sarchlab/akita/noc/v3 => ../noc

// replace gitlub.com/sarchlab/akita/v3/mem/ => ../mem

replace gitlub.com/sarchlab/akita/v3 => ../akita

go 1.18
