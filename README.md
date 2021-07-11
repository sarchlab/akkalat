# akkalat
Akkalat is the infrastructure to simulation wafer-scale GPUs. 

## CU-level mesh

### Synopsis

In this part, we abstract layers called **mesh** and **tile** between the level GPU and Shader Array. 

A **mesh** contains tileWidth \* tileHeight tiles (4*4 by default, mutable in gpu builder). And we implement each **tile** with a shader array (contains 4 CUs by default, each CU contains 1 L1 Caches and 1 L1 TLB).

Outside the mesh, we lay out the CP, L2 Caches, L2 TLB, etc. `periphPorts` is used to connect ports to mesh in tile[0, 0], and `periphConn` is to directly connect the components which completely insulates from mesh. The detailed descriptions of the connections can be found in gpu.go comments.

Thus, by default the number of main components like shader arrays, CUs, TLBs and Caches is exactly the same with R9 Nano GPU.

### Build

```bash
MGPUSIM_DIR=/path/to/your/mgpusim
# Please be careful about the potential overriding operation, 
# although `-b` options would backup the old files in runner
mv -b -v runner/* $MGPUSIM_DIR/samples/runner
cd $MGPUSIM_DIR/fft
go build && ./fft -timing -report-all -parallel -verify
```
