module github.com/sarchlab/akkalat

require (
	github.com/sarchlab/akita/v3 v3.0.0
	github.com/sarchlab/mgpusim/v3 v3.0.0-20230622042936-16aa3c53211e
	github.com/tebeka/atexit v0.3.0
)

require filippo.io/edwards25519 v1.1.0 // indirect

require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-sql-driver/mysql v1.8.0 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/google/pprof v0.0.0-20240227163752-401108e1b7e7 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/rs/xid v1.5.0 // indirect
	// github.com/sarchlab/mgpusim/v3 v3.0.0-20230620043528-e67cf84c1c45 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/syifan/goseth v0.1.2 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sys v0.18.0 // indirect
	gonum.org/v1/gonum v0.14.0 // indirect
)

replace github.com/sarchlab/mgpusim/v3 => ../../mgpusim

replace github.com/sarchlab/akita/v3 => ../../akita

go 1.22

toolchain go1.22.4
