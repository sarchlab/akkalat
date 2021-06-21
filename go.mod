module github.com/sarchlab/akkalab

require (
	github.com/tebeka/atexit v0.3.0 // indirect
	gitlab.com/akita/akita/v2 v2.0.2
	gitlab.com/akita/mem/v2 v2.0.0 // indirect
	gitlab.com/akita/mgpusim/v2 v2.0.0 // indirect
	gitlab.com/akita/util/v2 v2.0.0 // indirect
)

replace github.com/sarchlab/akkalab/mgpu_config => ../mgpu_config

go 1.16
