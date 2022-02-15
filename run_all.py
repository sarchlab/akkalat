import subprocess
import os
from multiprocessing.pool import ThreadPool
import time

exps = [
    ("model1", "aes"),
    ("model1", "atax"),
    ("model1", "bicg"),
    ("model1", "bitonicsort"),
    ("model1", "conv2d"),
    ("model1", "fastwalshtransform"),
    ("model1", "fir"),
    ("model1", "fft"),
    ("model1", "floydwarshall"),
    ("model1", "im2col"),
    ("model1", "kmeans"),
    ("model1", "matrixmultiplication"),
    ("model1", "matrixtranspose"),
    ("model1", "nbody"),
    ("model1", "nw"),
    ("model1", "pagerank"),
    ("model1", "relu"),
    ("model1", "simpleconvolution"),
    ("model1", "spmv"),
    ("model1", "stencil2d"),
    ("model2", "aes"),
    ("model2", "atax"),
    ("model2", "bicg"),
    ("model2", "bitonicsort"),
    ("model2", "conv2d"),
    ("model2", "fastwalshtransform"),
    ("model2", "fir"),
    ("model2", "fft"),
    ("model2", "floydwarshall"),
    ("model2", "im2col"),
    ("model2", "kmeans"),
    ("model2", "matrixmultiplication"),
    ("model2", "matrixtranspose"),
    ("model2", "nbody"),
    ("model2", "nw"),
    ("model2", "pagerank"),
    ("model2", "relu"),
    ("model2", "simpleconvolution"),
    ("model2", "spmv"),
    ("model2", "stencil2d"),
]


def run_exp(exp):
    cwd = os.getcwd()
    metic_file_name = f'{exp[0]}_{exp[1]}_metrics'
    cmd = f'{cwd}/{exp[0]}/{exp[0]} -timing -magic-memory-copy -benchmark={exp[1]} -metric-file-name={metic_file_name}'
    print(cmd)

    out_file_name = f'{cwd}/{exp[0]}_{exp[1]}_out.stdout'

    out_file = open(out_file_name, "w")
    out_file.write(f'Executing {cmd}\n')
    start_time = time.Now()
    out_file.write(f'Start time: {start_time}\n')
    out_file.flush()

    process = subprocess.Popen(
        cmd, shell=True, stdout=out_file, stderr=out_file, cwd=cwd)
    process.wait()

    if process.returncode != 0:
        print("Error executing ", cmd)
    else:
        print("Executed ", cmd)

    end_time = time.Now()
    out_file.write(f'End time: {end_time}\n')

    out_file.close()


def main():
    cwd = os.getcwd()

    process = subprocess.Popen("cd model1 && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen("cd model2 && go build", shell=True, cwd=cwd)
    process.wait()

    tp = ThreadPool()
    for exp in exps:
        tp.apply_async(run_exp, args=(exp, ))

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
