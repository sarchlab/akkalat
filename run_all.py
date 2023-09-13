import subprocess
import os
from multiprocessing.pool import ThreadPool
from datetime import datetime

exps = [
    # ("32CUPerGPU_withoutCache", "atax", []),
    # ("32CUPerGPU_withoutCache", "bicg", []),
    # ("32CUPerGPU_withoutCache", "bitonicsort", []),
    # ("32CUPerGPU_withoutCache", "fastwalshtransform", []),
    # ("32CUPerGPU_withoutCache", "fir", []),
    # ("32CUPerGPU_withoutCache", "fft", []),
    # ("32CUPerGPU_withoutCache", "im2col", []),
    # ("32CUPerGPU_withoutCache", "kmeans", []),
    # ("32CUPerGPU_withoutCache", "matrixmultiplication", []),
    # ("32CUPerGPU_withoutCache", "matrixtranspose", []),
    # ("32CUPerGPU_withoutCache", "nbody", []),
    # ("32CUPerGPU_withoutCache", "nw", []),
    # ("32CUPerGPU_withoutCache", "pagerank", []),
    # ("32CUPerGPU_withoutCache", "relu", []),
    # ("32CUPerGPU_withoutCache", "spmv", []),

    # ("64CUPerGPU_withCache", "atax", ["-analyzer-Name=port.csv",
    #             "-analyzer-period=1e-6"]),
    # ("64CUPerGPU_withoutCache", "bicg", []),
    # ("64CUPerGPU_withoutCache", "bitonicsort", []),
    # ("64CUPerGPU_withoutCache", "fastwalshtransform", []),
    # ("64CUPerGPU_withCache", "fir", ["-analyzer-Name=firport -analyzer-period=1e-6"]),
    # ("64CUPerGPU_withoutCache", "fft", []),
    # ("64CUPerGPU_withoutCache", "im2col", []),
    # ("64CUPerGPU_withoutCache", "kmeans", []),
    # ("64CUPerGPU_withoutCache", "matrixmultiplication", []),
    # ("64CUPerGPU_withoutCache", "matrixtranspose", []),
    # ("64CUPerGPU_withoutCache", "nbody", []),
    # ("64CUPerGPU_withoutCache", "nw", []),
    # ("64CUPerGPU_withoutCache", "pagerank", []),
    # ("64CUPerGPU_withoutCache", "relu", []),
    # ("64CUPerGPU_withoutCache", "spmv", []),   

    # ("128CUPerGPU_withoutCache", "atax", []),
    # ("128CUPerGPU_withoutCache", "bicg", []),
    # ("128CUPerGPU_withoutCache", "bitonicsort", []),
    # ("128CUPerGPU_withoutCache", "fastwalshtransform", []),
    # ("128CUPerGPU_withoutCache", "fir", []),
    # ("128CUPerGPU_withoutCache", "fft", []),
    # ("128CUPerGPU_withoutCache", "im2col", []),
    # ("128CUPerGPU_withoutCache", "kmeans", []),
    # ("128CUPerGPU_withoutCache", "matrixmultiplication", []),
    # ("128CUPerGPU_withoutCache", "matrixtranspose", []),
    # ("128CUPerGPU_withoutCache", "nbody", []),
    # ("128CUPerGPU_withoutCache", "nw", []),
    # ("128CUPerGPU_withoutCache", "pagerank", []),
    # ("128CUPerGPU_withoutCache", "relu", []),
    # ("128CUPerGPU_withoutCache", "spmv", []),   

    # ("256CUPerGPU_withCache", "atax", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bicg", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fir", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fft", [" -num-memory-banks=64 -bandwidth=384"]),
    ("256CUPerGPU_withCache", "im2col", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "kmeans", [" -num-memory-banks=64 -bandwidth=384"]),
    ("256CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=64 -bandwidth=384"]),
    ("256CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "nbody", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "nw", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "pagerank", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "relu", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "spmv", [" -num-memory-banks=64 -bandwidth=384"]),
]

output_dir = ""


def run_exp(exp):
    try:
        cwd = os.getcwd()
        file_name = f'{output_dir}/{exp[0]}_{exp[1]}_{"_".join(exp[2])}'
        metic_file_name = file_name + ' _metrics'
        cmd = f"{cwd}/{exp[0]}/{exp[0]} -benchmark={exp[1]} -timing " + \
            "-magic-memory-copy -report-all " + \
            "-num-memory-banks=64 -bandwidth=384 " +\
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
    output_dir = f'results/{datetime.now().strftime("%Y-%m-%d-%H-%M-%S_memory_bank_cnanged")}'

    if not os.path.exists('results'):
        os.makedirs('results')

    if not os.path.exists(output_dir):
        os.makedirs(output_dir)


def main():
    cwd = os.getcwd()

    create_output_dir()

    process = subprocess.Popen("cd 64CUPerGPU_withCache && go build", shell=True, cwd=cwd)
    process.wait()

    # process = subprocess.Popen(
    #     "cd model1_separate_cache && go build", shell=True, cwd=cwd)
    # process.wait()

    # process = subprocess.Popen("cd model2 && go build", shell=True, cwd=cwd)
    # process.wait()

    # process = subprocess.Popen("cd model3 && go build", shell=True, cwd=cwd)
    # process.wait()

    tp = ThreadPool(16)
    for exp in exps:
        tp.apply_async(run_exp, args=(exp, ))

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
