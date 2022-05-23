# Akkalat

Akkalat is a wafer-scale GPU simulator.

## Model Definition

### Model 1

Model 1 is a simple mesh-based multi-GPU model. By default, model 1 configures a 5x5 mesh, with a CPU in the center and 24 GPUs around the CPU. Each GPU has 32 compute units and 4GB HBM DRAM. The communication bandwidth between adjacent GPUs is 16GB/s.

The design is depicted below:

![Model 1 Design](model1.drawio.png)

### Model 2

Model 2 is similar to model 1. The only difference is that the underlying network is replaced with a high-bandwidth photonic network. It can be considered as a single hub (crossbar) that connects all the GPUs. The communication bandwidth between each GPU to the hub is 64GB/s with 1ns latency. The hub forwards the data to the destination GPU with a 2ns latency.

### Model 3

Model 3 disaggregates the memory from the GPU tiles. All the periphral tiles are pure GPU tiles that are equipped with 48 CUs each. The 9 tiles between the CPU tile and the GPU tiles are pure memory nodes. The network is the same as in model 1.