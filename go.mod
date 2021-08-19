module github.com/sarchlab/akkalat

require (
	github.com/tebeka/atexit v0.3.0
	gitlab.com/akita/akita/v2 v2.0.1
	gitlab.com/akita/mem/v2 v2.3.1
	gitlab.com/akita/mgpusim/v2 v2.0.0
	gitlab.com/akita/noc/v2 v2.0.0
	gitlab.com/akita/util/v2 v2.0.1
)

replace gitlab.com/akita/mgpusim/v2 => ../mgpusim

go 1.16
