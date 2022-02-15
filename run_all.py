import subprocess
import os
from multiprocessing.pool import ThreadPool

from sympy import EX

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
]


def run_exp(exp):
    cwd = os.getcwd()
    metic_file_name = f'{exp[0]}_{exp[1]}_metrics'
    cmd = f'{cwd}/{exp[0]}/{exp[0]} -timing -magic-memory-copy -benchmark={exp[1]} -metric-file-name={metic_file_name}'
    print(cmd)

    out_file_name = f'{cwd}/{exp[0]}_{exp[1]}_out.stdout'

    out_file = open(out_file_name, "w")
    out_file.write(f'Executing {cmd}\n')
    out_file.flush()

    process = subprocess.Popen(
        cmd, shell=True, stdout=out_file, stderr=out_file, cwd=cwd)
    process.wait()

    if process.returncode != 0:
        print("Error executing ", cmd)
    else:
        print("Executed ", cmd)

    out_file.close()


def main():
    subprocess.run(["go", "build", "./..."])

    tp = ThreadPool()
    for exp in exps:
        tp.apply_async(run_exp, args=(exp, ))

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
