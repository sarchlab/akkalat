import subprocess
import os
from multiprocessing.pool import ThreadPool
from datetime import datetime

exps = [
    ("16CUPerGPU_withoutCache", "aes", []),
    ("16CUPerGPU_withoutCache", "atax", []),
    ("16CUPerGPU_withoutCache", "bicg", []),
    ("16CUPerGPU_withoutCache", "bitonicsort", []),
    ("16CUPerGPU_withoutCache", "conv2d", []),
    ("16CUPerGPU_withoutCache", "fastwalshtransform", []),
    ("16CUPerGPU_withoutCache", "fir", []),
    ("16CUPerGPU_withoutCache", "fft", []),
    ("16CUPerGPU_withoutCache", "floydwarshall", []),
    ("16CUPerGPU_withoutCache", "im2col", []),
    ("16CUPerGPU_withoutCache", "kmeans", []),
    ("16CUPerGPU_withoutCache", "matrixmultiplication", []),
    ("16CUPerGPU_withoutCache", "matrixtranspose", []),
    ("16CUPerGPU_withoutCache", "nbody", []),
    ("16CUPerGPU_withoutCache", "nw", []),
    ("16CUPerGPU_withoutCache", "pagerank", []),
    ("16CUPerGPU_withoutCache", "relu", []),
    ("16CUPerGPU_withoutCache", "simpleconvolution", []),
    ("16CUPerGPU_withoutCache", "spmv", []),
    ("16CUPerGPU_withoutCache", "stencil2d", []),
    ("16CUPerGPU_withoutCache", "nbody", []),
    ("16CUPerGPU_withoutCache", "nw", []),
    ("16CUPerGPU_withoutCache", "pagerank", [])    
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
