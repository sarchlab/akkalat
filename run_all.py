import subprocess
import os
from multiprocessing.pool import ThreadPool
from datetime import datetime
import concurrent.futures

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

    # ("64CUPerGPU_withCache", "atax", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "bicg", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "fir", ["-analyzer-Name=firport -analyzer-period=1e-6"]),
    # ("64CUPerGPU_withoutCache", "fft", []),
    # ("64CUPerGPU_withoutCache", "im2col", []),
    # ("64CUPerGPU_withCache", "kmeans", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withoutCache", "nbody", []),
    # ("64CUPerGPU_withoutCache", "nw", []),
    # ("64CUPerGPU_withCache", "pagerank", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "relu", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "spmv", [" -num-memory-banks=16 -bandwidth=96"]), 

    # ("4CUPerGPU_withCache", "fft", [" -num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "atax", [" -num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "bicg", ["-num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "bitonicsort", ["-num-memory-banks=1 -bandwidth=6"]), 
    # ("4CUPerGPU_withCache", "kmeans", [" -num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=1 -bandwidth=6"]),
    # ("4CUPerGPU_withCache", "spmv", [" -num-memory-banks=1 -bandwidth=6"]), 

    # ("8CUPerGPU_withCache", "fft", [" -num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "atax", [" -num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "bicg", ["-num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "bitonicsort", ["-num-memory-banks=2 -bandwidth=12"]), 
    # ("8CUPerGPU_withCache", "kmeans", [" -num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=2 -bandwidth=12"]),
    # ("8CUPerGPU_withCache", "spmv", [" -num-memory-banks=2 -bandwidth=12"]), 

    # ("16CUPerGPU_withCache", "fft", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "atax", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "bicg", ["-num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "bitonicsort", ["-num-memory-banks=4 -bandwidth=24"]), 
    # ("16CUPerGPU_withCache", "kmeans", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "spmv", [" -num-memory-banks=4 -bandwidth=24"]), 

    # ("32CUPerGPU_withCache", "fft", [" -num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "atax", [" -num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "bicg", ["-num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "bitonicsort", ["-num-memory-banks=8 -bandwidth=48"]), 
    # ("32CUPerGPU_withCache", "kmeans", [" -num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=8 -bandwidth=48"]),
    # ("32CUPerGPU_withCache", "spmv", [" -num-memory-banks=8 -bandwidth=48"]), 

    # ("64CUPerGPU_withCache", "fft", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "im2col", [" -num-memory-banks=16 -bandwidth=96"]), 
    # ("64CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "atax", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "bicg", ["-num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "bitonicsort", ["-num-memory-banks=16 -bandwidth=96"]), 
    # ("64CUPerGPU_withCache", "kmeans", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "spmv", [" -num-memory-banks=16 -bandwidth=96"]), 
   

    # ("128CUPerGPU_withCache", "fft", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "atax", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "bicg", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=32 -bandwidth=192"]), 
    # ("128CUPerGPU_withCache", "kmeans", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "spmv", [" -num-memory-banks=32 -bandwidth=192"]), 

    # ("256CUPerGPU_withCache", "fft", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "atax", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bicg", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=64 -bandwidth=384"]), 
    # ("256CUPerGPU_withCache", "kmeans", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "pagerank", [" -num-memory-banks=64 -bandwidth=384"]), 

    # ("16CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "im2col", ["-num-memory-banks=4 -bandwidth=24 "]),
    # ("8CUPerGPU_withCache", "im2col", ["-num-memory-banks=2 -bandwidth=12 "]),
    # ("32CUPerGPU_withCache", "relu", ["-num-memory-banks=8 -bandwidth=48 "]),
    # ("16CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "pagerank", [" -num-memory-banks=4 -bandwidth=24"]),
    # ("16CUPerGPU_withCache", "relu", [" -num-memory-banks=4 -bandwidth=24"]), 

    # ("32CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=8 -bandwidth=48"]),
    # # ("64CUPerGPU_withCache", "im2col", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("32CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=8 -bandwidth=48"]),
    # # ("64CUPerGPU_withCache", "pagerank", [" -num-memory-banks=16 -bandwidth=96"]),
    # # ("64CUPerGPU_withCache", "relu", [" -num-memory-banks=16 -bandwidth=96"]), 

    # ("64CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "im2col", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "pagerank", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "relu", [" -num-memory-banks=16 -bandwidth=96"]), 

    # ("128CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "im2col", ["-num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "matrixtranspose", ["-num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "pagerank", ["-num-memory-banks=32 -bandwidth=192"]),
    # ("128CUPerGPU_withCache", "relu", ["-num-memory-banks=32 -bandwidth=192"]), 

    # ("256CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "im2col", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "pagerank", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "relu", [" -num-memory-banks=64 -bandwidth=384"]),

     # ("256CUPerGPU_withCache", "atax", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bicg", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fir", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "fft", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "im2col", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "kmeans", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "nbody", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "nw", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "pagerank", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "relu", [" -num-memory-banks=64 -bandwidth=384"]),
    # ("256CUPerGPU_withCache", "spmv", [" -num-memory-banks=64 -bandwidth=384"]),

    ("64CUPerGPU_withCache", "atax", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "bicg", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "bitonicsort", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "fastwalshtransform", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "fir", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "fft", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "im2col", [" -num-memory-banks=16 -bandwidth=96"]),
    # ("64CUPerGPU_withCache", "kmeans", [" -num-memory-banks=16 -bandwidth=384"]),
    ("64CUPerGPU_withCache", "matrixmultiplication", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "matrixtranspose", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "nbody", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "nw", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "pagerank", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "relu", [" -num-memory-banks=16 -bandwidth=96"]),
    ("64CUPerGPU_withCache", "spmv", [" -num-memory-banks=16 -bandwidth=96"]),

]

output_dir = ""


def run_exp(exp):
    try:
        cwd = os.getcwd()
        file_name = f'{output_dir}/{exp[0]}_{exp[1]}_{"_".join(exp[2])}'
        metic_file_name = file_name + ' _metrics'
        cmd = f"{cwd}/{exp[0]}/{exp[0]} -benchmark={exp[1]} -timing " + \
            "-magic-memory-copy -report-all " + \
            " ".join(exp[2]) + " " + \
            f"-metric-file-name={metic_file_name}" + \
            "".join(exp[2])
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
    output_dir = f'results/{datetime.now().strftime("%Y-%m-%d-%H-%M-%SExperiment2GPMNew4x5")}'

    if not os.path.exists('results'):
        os.makedirs('results')

    if not os.path.exists(output_dir):
        os.makedirs(output_dir)


def main():
    cwd = os.getcwd()

    create_output_dir()

    process = subprocess.Popen("cd 64CUPerGPU_withCache && go build", shell=True, cwd=cwd)
    process.wait()

    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:
        future = [executor.submit(run_exp, exp) for exp in exps]

        for future in concurrent.futures.as_completed(future):
            print(future.result())

    tp.close()
    tp.join()


if __name__ == "__main__":
    main()
