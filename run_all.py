import subprocess
import os
from multiprocessing.pool import ThreadPool
from datetime import datetime

exps = [
    ("model1", "aes", ['-switch-latency=1']),
    ("model1", "atax", ['-switch-latency=1']),
    ("model1", "bicg", ['-switch-latency=1']),
    ("model1", "bitonicsort", ['-switch-latency=1']),
    ("model1", "conv2d", ['-switch-latency=1']),
    ("model1", "fastwalshtransform", ['-switch-latency=1']),
    ("model1", "fir", ['-switch-latency=1']),
    ("model1", "fft", ['-switch-latency=1']),
    ("model1", "floydwarshall", ['-switch-latency=1']),
    ("model1", "im2col", ['-switch-latency=1']),
    ("model1", "kmeans", ['-switch-latency=1']),
    ("model1", "matrixmultiplication", ['-switch-latency=1']),
    ("model1", "matrixtranspose", ['-switch-latency=1']),
    ("model1", "nbody", ['-switch-latency=1']),
    ("model1", "nw", ['-switch-latency=1']),
    ("model1", "pagerank", ['-switch-latency=1']),
    ("model1", "relu", ['-switch-latency=1']),
    ("model1", "simpleconvolution", ['-switch-latency=1']),
    ("model1", "spmv", ['-switch-latency=1']),
    ("model1", "stencil2d", ['-switch-latency=1']),
    ("model1", "aes", ['-switch-latency=2']),
    ("model1", "atax", ['-switch-latency=2']),
    ("model1", "bicg", ['-switch-latency=2']),
    ("model1", "bitonicsort", ['-switch-latency=2']),
    ("model1", "conv2d", ['-switch-latency=2']),
    ("model1", "fastwalshtransform", ['-switch-latency=2']),
    ("model1", "fir", ['-switch-latency=2']),
    ("model1", "fft", ['-switch-latency=2']),
    ("model1", "floydwarshall", ['-switch-latency=2']),
    ("model1", "im2col", ['-switch-latency=2']),
    ("model1", "kmeans", ['-switch-latency=2']),
    ("model1", "matrixmultiplication", ['-switch-latency=2']),
    ("model1", "matrixtranspose", ['-switch-latency=2']),
    ("model1", "nbody", ['-switch-latency=2']),
    ("model1", "nw", ['-switch-latency=2']),
    ("model1", "pagerank", ['-switch-latency=2']),
    ("model1", "relu", ['-switch-latency=2']),
    ("model1", "simpleconvolution", ['-switch-latency=2']),
    ("model1", "spmv", ['-switch-latency=2']),
    ("model1", "stencil2d", ['-switch-latency=2']),
    ("model1", "aes", ['-switch-latency=5']),
    ("model1", "atax", ['-switch-latency=5']),
    ("model1", "bicg", ['-switch-latency=5']),
    ("model1", "bitonicsort", ['-switch-latency=5']),
    ("model1", "conv2d", ['-switch-latency=5']),
    ("model1", "fastwalshtransform", ['-switch-latency=5']),
    ("model1", "fir", ['-switch-latency=5']),
    ("model1", "fft", ['-switch-latency=5']),
    ("model1", "floydwarshall", ['-switch-latency=5']),
    ("model1", "im2col", ['-switch-latency=5']),
    ("model1", "kmeans", ['-switch-latency=5']),
    ("model1", "matrixmultiplication", ['-switch-latency=5']),
    ("model1", "matrixtranspose", ['-switch-latency=5']),
    ("model1", "nbody", ['-switch-latency=5']),
    ("model1", "nw", ['-switch-latency=5']),
    ("model1", "pagerank", ['-switch-latency=5']),
    ("model1", "relu", ['-switch-latency=5']),
    ("model1", "simpleconvolution", ['-switch-latency=5']),
    ("model1", "spmv", ['-switch-latency=5']),
    ("model1", "stencil2d", ['-switch-latency=5']),
    ("model1", "aes", ['-switch-latency=10']),
    ("model1", "atax", ['-switch-latency=10']),
    ("model1", "bicg", ['-switch-latency=10']),
    ("model1", "bitonicsort", ['-switch-latency=10']),
    ("model1", "conv2d", ['-switch-latency=10']),
    ("model1", "fastwalshtransform", ['-switch-latency=10']),
    ("model1", "fir", ['-switch-latency=10']),
    ("model1", "fft", ['-switch-latency=10']),
    ("model1", "floydwarshall", ['-switch-latency=10']),
    ("model1", "im2col", ['-switch-latency=10']),
    ("model1", "kmeans", ['-switch-latency=10']),
    ("model1", "matrixmultiplication", ['-switch-latency=10']),
    ("model1", "matrixtranspose", ['-switch-latency=10']),
    ("model1", "nbody", ['-switch-latency=10']),
    ("model1", "nw", ['-switch-latency=10']),
    ("model1", "pagerank", ['-switch-latency=10']),
    ("model1", "relu", ['-switch-latency=10']),
    ("model1", "simpleconvolution", ['-switch-latency=10']),
    ("model1", "spmv", ['-switch-latency=10']),
    ("model1", "stencil2d", ['-switch-latency=10']),
    ("model1", "aes", ['-switch-latency=20']),
    ("model1", "atax", ['-switch-latency=20']),
    ("model1", "bicg", ['-switch-latency=20']),
    ("model1", "bitonicsort", ['-switch-latency=20']),
    ("model1", "conv2d", ['-switch-latency=20']),
    ("model1", "fastwalshtransform", ['-switch-latency=20']),
    ("model1", "fir", ['-switch-latency=20']),
    ("model1", "fft", ['-switch-latency=20']),
    ("model1", "floydwarshall", ['-switch-latency=20']),
    ("model1", "im2col", ['-switch-latency=20']),
    ("model1", "kmeans", ['-switch-latency=20']),
    ("model1", "matrixmultiplication", ['-switch-latency=20']),
    ("model1", "matrixtranspose", ['-switch-latency=20']),
    ("model1", "nbody", ['-switch-latency=20']),
    ("model1", "nw", ['-switch-latency=20']),
    ("model1", "pagerank", ['-switch-latency=20']),
    ("model1", "relu", ['-switch-latency=20']),
    ("model1", "simpleconvolution", ['-switch-latency=20']),
    ("model1", "spmv", ['-switch-latency=20']),
    ("model1", "stencil2d", ['-switch-latency=20']),
    ("model1", "aes", ['-switch-latency=50']),
    ("model1", "atax", ['-switch-latency=50']),
    ("model1", "bicg", ['-switch-latency=50']),
    ("model1", "bitonicsort", ['-switch-latency=50']),
    ("model1", "conv2d", ['-switch-latency=50']),
    ("model1", "fastwalshtransform", ['-switch-latency=50']),
    ("model1", "fir", ['-switch-latency=50']),
    ("model1", "fft", ['-switch-latency=50']),
    ("model1", "floydwarshall", ['-switch-latency=50']),
    ("model1", "im2col", ['-switch-latency=50']),
    ("model1", "kmeans", ['-switch-latency=50']),
    ("model1", "matrixmultiplication", ['-switch-latency=50']),
    ("model1", "matrixtranspose", ['-switch-latency=50']),
    ("model1", "nbody", ['-switch-latency=50']),
    ("model1", "nw", ['-switch-latency=50']),
    ("model1", "pagerank", ['-switch-latency=50']),
    ("model1", "relu", ['-switch-latency=50']),
    ("model1", "simpleconvolution", ['-switch-latency=50']),
    ("model1", "spmv", ['-switch-latency=50']),
    ("model1", "stencil2d", ['-switch-latency=50']),
    # ("model2", "aes"),
    # ("model2", "atax"),
    # ("model2", "bicg"),
    # ("model2", "bitonicsort"),
    # ("model2", "conv2d"),
    # ("model2", "fastwalshtransform"),
    # ("model2", "fir"),
    # ("model2", "fft"),
    # ("model2", "floydwarshall"),
    # ("model2", "im2col"),
    # ("model2", "kmeans"),
    # ("model2", "matrixmultiplication"),
    # ("model2", "matrixtranspose"),
    # ("model2", "nbody"),
    # ("model2", "nw"),
    # ("model2", "pagerank"),
    # ("model2", "relu"),
    # ("model2", "simpleconvolution"),
    # ("model2", "spmv"),
    # ("model2", "stencil2d"),
    # ("model3", "aes"),
    # ("model3", "atax"),
    # ("model3", "bicg"),
    # ("model3", "bitonicsort"),
    # ("model3", "conv2d"),
    # ("model3", "fastwalshtransform"),
    # ("model3", "fir"),
    # ("model3", "fft"),
    # ("model3", "floydwarshall"),
    # ("model3", "im2col"),
    # ("model3", "kmeans"),
    # ("model3", "matrixmultiplication"),
    # ("model3", "matrixtranspose"),
    # ("model3", "nbody"),
    # ("model3", "nw"),
    # ("model3", "pagerank"),
    # ("model3", "relu"),
    # ("model3", "simpleconvolution"),
    # ("model3", "spmv"),
    # ("model3", "stencil2d"),
]


def run_exp(exp):
    try:
        cwd = os.getcwd()
        file_name = f'{exp[0]}_{exp[1]}_{"_".join(exp[2])}'
        metic_file_name = file_name + '_metrics'
        cmd = f"{cwd}/{exp[0]}/{exp[0]} -benchmark={exp[1]} -timing " + \
            "-magic-memory-copy -report-all " + \
            f"-metric-file-name={metic_file_name} " + \
            " ".join(exp[2])
        print(cmd)

        out_file_name = file_name+'_out.stdout'

        out_file = open(out_file_name, "w")
        out_file.write(f'Executing {cmd}\n')
        start_time = datetime.now()
        out_file.write(f'Start time: {start_time}\n')
        out_file.flush()

        process = subprocess.Popen(
            cmd, shell=True, stdout=out_file, stderr=out_file, cwd=cwd)
        process.wait()

        end_time = datetime.now()
        out_file.write(f'End time: {end_time}\n')

        elapsed_time = end_time - start_time
        out_file.write(f'Elapsed time: {elapsed_time}\n')

        if process.returncode != 0:
            print("Error executing ", cmd)
        else:
            print("Executed ", cmd, ", time ", elapsed_time)

        out_file.close()
    except Exception as e:
        print(e)


def main():
    cwd = os.getcwd()

    process = subprocess.Popen("cd model1 && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen("cd model2 && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen("cd model3 && go build", shell=True, cwd=cwd)
    process.wait()

    tp = ThreadPool(32)
    for exp in exps:
        tp.apply_async(run_exp, args=(exp, ))

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
