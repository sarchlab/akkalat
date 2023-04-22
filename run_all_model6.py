import subprocess
import os
from multiprocessing.pool import ThreadPool
from datetime import datetime

exps = [
    ("model1", "kmeans", ['-num-memory-banks=4']),
    ("model1", "matrixmultiplication", ['-num-memory-banks=4']),
    ("model1", "matrixtranspose", ['-num-memory-banks=4']),
    # ("model1", "nbody", ['-num-memory-banks=4']),
    # ("model1", "nw", ['-num-memory-banks=4']),
    # ("model1", "pagerank", ['-num-memory-banks=4']),

    # ("model1", "aes", []),
    # ("model1", "atax", []),
    # ("model1", "bicg", []),
    # ("model1", "bitonicsort", []),
    # ("model1", "conv2d", []),
    # ("model1", "fastwalshtransform", []),
    # ("model1", "fir", []),
    # ("model1", "fft", []),
    # ("model1", "floydwarshall", []),
    # ("model1", "im2col", []),
    # ("model1", "kmeans", []),
    # ("model1", "matrixmultiplication", []),
    # ("model1", "matrixtranspose", []),
    # ("model1", "nbody", []),
    # ("model1", "nw", []),
    # ("model1", "pagerank", []),
    # ("model1", "relu", []),
    # ("model1", "simpleconvolution", []),
    # ("model1", "spmv", []),
    # ("model1", "stencil2d", []),

    # ("model6_48_Small_GPU", "aes", []),
    # ("model6_48_Small_GPU", "atax", []),
    # ("model6_48_Small_GPU", "bicg", []),
    # ("model6_48_Small_GPU", "bitonicsort", []),
    # ("model6_48_Small_GPU", "conv2d", []),
    # ("model6_48_Small_GPU", "fastwalshtransform", []),
    # ("model6_48_Small_GPU", "fir", []),
    # ("model6_48_Small_GPU", "fft", []),
    # ("model6_48_Small_GPU", "floydwarshall", []),
    # ("model6_48_Small_GPU", "im2col", []),
    # ("model6_48_Small_GPU", "kmeans", []),
    # ("model6_48_Small_GPU", "matrixmultiplication", []),
    # # ("model6_48_Small_GPU", "matrixtranspose", []),
    # # ("model6_48_Small_GPU", "nbody", []),
    # ("model6_48_Small_GPU", "nw", []),
    # ("model6_48_Small_GPU", "pagerank", []),
    # ("model6_48_Small_GPU", "relu", []),
    # ("model6_48_Small_GPU", "simpleconvolution", []),
    # ("model6_48_Small_GPU", "spmv", []),
    # ("model6_48_Small_GPU", "stencil2d", []),



    # ("model1_separate_cache", "aes", ['-switch-latency=1']),
    # ("model1_separate_cache", "atax", ['-switch-latency=1']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=1']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=1']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=1']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=1']),
    # ("model1_separate_cache", "fir", ['-switch-latency=1']),
    # ("model1_separate_cache", "fft", ['-switch-latency=1']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=1']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=1']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=1']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=1']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=1']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=1']),
    # ("model1_separate_cache", "nw", ['-switch-latency=1']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=1']),
    # ("model1_separate_cache", "relu", ['-switch-latency=1']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=1']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=1']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=1']),
    # ("model1_separate_cache", "aes", ['-switch-latency=2']),
    # ("model1_separate_cache", "atax", ['-switch-latency=2']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=2']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=2']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=2']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=2']),
    # ("model1_separate_cache", "fir", ['-switch-latency=2']),
    # ("model1_separate_cache", "fft", ['-switch-latency=2']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=2']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=2']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=2']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=2']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=2']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=2']),
    # ("model1_separate_cache", "nw", ['-switch-latency=2']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=2']),
    # ("model1_separate_cache", "relu", ['-switch-latency=2']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=2']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=2']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=2']),
    # ("model1_separate_cache", "aes", ['-switch-latency=5']),
    # ("model1_separate_cache", "atax", ['-switch-latency=5']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=5']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=5']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=5']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=5']),
    # ("model1_separate_cache", "fir", ['-switch-latency=5']),
    # ("model1_separate_cache", "fft", ['-switch-latency=5']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=5']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=5']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=5']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=5']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=5']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=5']),
    # ("model1_separate_cache", "nw", ['-switch-latency=5']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=5']),
    # ("model1_separate_cache", "relu", ['-switch-latency=5']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=5']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=5']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=5']),
    # ("model1_separate_cache", "aes", ['-switch-latency=10']),
    # ("model1_separate_cache", "atax", ['-switch-latency=10']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=10']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=10']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=10']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=10']),
    # ("model1_separate_cache", "fir", ['-switch-latency=10']),
    # ("model1_separate_cache", "fft", ['-switch-latency=10']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=10']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=10']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=10']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=10']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=10']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=10']),
    # ("model1_separate_cache", "nw", ['-switch-latency=10']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=10']),
    # ("model1_separate_cache", "relu", ['-switch-latency=10']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=10']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=10']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=10']),
    # ("model1_separate_cache", "aes", ['-switch-latency=20']),
    # ("model1_separate_cache", "atax", ['-switch-latency=20']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=20']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=20']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=20']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=20']),
    # ("model1_separate_cache", "fir", ['-switch-latency=20']),
    # ("model1_separate_cache", "fft", ['-switch-latency=20']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=20']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=20']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=20']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=20']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=20']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=20']),
    # ("model1_separate_cache", "nw", ['-switch-latency=20']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=20']),
    # ("model1_separate_cache", "relu", ['-switch-latency=20']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=20']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=20']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=20']),
    # ("model1_separate_cache", "aes", ['-switch-latency=50']),
    # ("model1_separate_cache", "atax", ['-switch-latency=50']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=50']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=50']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=50']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=50']),
    # ("model1_separate_cache", "fir", ['-switch-latency=50']),
    # ("model1_separate_cache", "fft", ['-switch-latency=50']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=50']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=50']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=50']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=50']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=50']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=50']),
    # ("model1_separate_cache", "nw", ['-switch-latency=50']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=50']),
    # ("model1_separate_cache", "relu", ['-switch-latency=50']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=50']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=50']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=50']),
    # ("model1_separate_cache", "aes", ['-switch-latency=100']),
    # ("model1_separate_cache", "atax", ['-switch-latency=100']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=100']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=100']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=100']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=100']),
    # ("model1_separate_cache", "fir", ['-switch-latency=100']),
    # ("model1_separate_cache", "fft", ['-switch-latency=100']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=100']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=100']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=100']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=100']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=100']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=100']),
    # ("model1_separate_cache", "nw", ['-switch-latency=100']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=100']),
    # ("model1_separate_cache", "relu", ['-switch-latency=100']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=100']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=100']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=100']),

    # ("model1_separate_cache", "aes", ['-switch-latency=200']),
    # ("model1_separate_cache", "atax", ['-switch-latency=200']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=200']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=200']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=200']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=200']),
    # ("model1_separate_cache", "fir", ['-switch-latency=200']),
    # ("model1_separate_cache", "fft", ['-switch-latency=200']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=200']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=200']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=200']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=200']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=200']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=200']),
    # ("model1_separate_cache", "nw", ['-switch-latency=200']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=200']),
    # ("model1_separate_cache", "relu", ['-switch-latency=200']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=200']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=200']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=200']),

    # ("model1_separate_cache", "aes", ['-switch-latency=500']),
    # ("model1_separate_cache", "atax", ['-switch-latency=500']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=500']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=500']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=500']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=500']),
    # ("model1_separate_cache", "fir", ['-switch-latency=500']),
    # ("model1_separate_cache", "fft", ['-switch-latency=500']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=500']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=500']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=500']),
    # ("model1_separate_cache", "matrixmultiplication", ['-switch-latency=500']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=500']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=500']),
    # ("model1_separate_cache", "nw", ['-switch-latency=500']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=500']),
    # ("model1_separate_cache", "relu", ['-switch-latency=500']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=500']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=500']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=500']),

    # ("model1_separate_cache", "aes", ['-switch-latency=1000']),
    # ("model1_separate_cache", "atax", ['-switch-latency=1000']),
    # ("model1_separate_cache", "bicg", ['-switch-latency=1000']),
    # ("model1_separate_cache", "bitonicsort", ['-switch-latency=1000']),
    # ("model1_separate_cache", "conv2d", ['-switch-latency=1000']),
    # ("model1_separate_cache", "fastwalshtransform", ['-switch-latency=1000']),
    # ("model1_separate_cache", "fir", ['-switch-latency=1000']),
    # ("model1_separate_cache", "fft", ['-switch-latency=1000']),
    # ("model1_separate_cache", "floydwarshall", ['-switch-latency=1000']),
    # ("model1_separate_cache", "im2col", ['-switch-latency=1000']),
    # ("model1_separate_cache", "kmeans", ['-switch-latency=1000']),
    # ("model1_separate_cache", "matrixmultiplication",
    #  ['-switch-latency=1000']),
    # ("model1_separate_cache", "matrixtranspose", ['-switch-latency=1000']),
    # ("model1_separate_cache", "nbody", ['-switch-latency=1000']),
    # ("model1_separate_cache", "nw", ['-switch-latency=1000']),
    # ("model1_separate_cache", "pagerank", ['-switch-latency=1000']),
    # ("model1_separate_cache", "relu", ['-switch-latency=1000']),
    # ("model1_separate_cache", "simpleconvolution", ['-switch-latency=1000']),
    # ("model1_separate_cache", "spmv", ['-switch-latency=1000']),
    # ("model1_separate_cache", "stencil2d", ['-switch-latency=1000']),


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

    # ("model4_Mollca_GPU", "aes", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "atax", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "bicg", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "bitonicsort", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "conv2d", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "fastwalshtransform", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "fir", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "fft", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "floydwarshall", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "im2col", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "kmeans", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "matrixmultiplication", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "matrixtranspose", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "nbody", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "nw", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "pagerank", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "relu", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "simpleconvolution", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "spmv", ['-switch-latency=1']),
    # ("model4_Mollca_GPU", "stencil2d", ['-switch-latency=1']),

    # ("model7_1_CU_Per_GPU", "aes", ['-switch-latency=1']),
    # ("model7_1_CU_Per_GPU", "atax", ['-switch-latency=1']),
    #  ("model7_1_CU_Per_GPU", "relu", []),
    # ("model7_1_CU_Per_GPU", "bicg", []),
]

output_dir = ""


def run_exp(exp):
    try:
        cwd = os.getcwd()
        file_name = f'{output_dir}/{exp[0]}_{exp[1]}_{"_".join(exp[2])}'
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


def create_output_dir():
    global output_dir
    output_dir = f'results/{datetime.now().strftime("%Y-%m-%d-%H-%M-%S")}'

    if not os.path.exists('results'):
        os.makedirs('results')

    if not os.path.exists(output_dir):
        os.makedirs(output_dir)


def main():
    cwd = os.getcwd()

    create_output_dir()

    process = subprocess.Popen("cd model1 && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen(
        "cd model1_separate_cache && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen("cd model2 && go build", shell=True, cwd=cwd)
    process.wait()

    process = subprocess.Popen("cd model3 && go build", shell=True, cwd=cwd)
    process.wait()

    tp = ThreadPool(16)
    for exp in exps:
        tp.apply_async(run_exp, args=(exp, ))

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
