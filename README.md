# Distributed Training Tools for Large Models

A distributed training tool for large models, composed of three collaborative components: **Master Control**, **Data Node**, and **Training Node**.

---

## 1. Introduction

| Component      | Directory         | Programming language | Responsibilities                                                                  |
|----------------|-------------------|----------------------|------------------------------------------------------------------------------------|
| Master Control | `Training/`       | Go                   | Task scheduling + interactive command-line console + task log collection/presentation |
| Data Node      | `DatabasePlugin/` | Go                   | Read data from MySQL, preprocess it into tensors, and stream it to the training side |
| Training Node  | `TrainPlugin/`    | Python               | Receive tasks and data, train using PyTorch, and save the model                    |

- **Master Control** (`Training`): an interactive command-line console that parses the user's task configuration and sends it to the training and data terminals to start the task; it runs the `RemoteLog` gRPC service (`:50053`) and collects the statuses reported by the data and training terminals.
- **Data Node** (`DatabasePlugin`): provides the external `DatabaseLink` gRPC service (`:50051`), reads raw samples from MySQL, preprocesses them into tensors (image/text/mixed), and sends them to the training side via `WithTrain`.
- **Training Node** (`TrainPlugin`): provides two external gRPC services, `Training` (`:50052`) and `WithTrain` (`:50054`), executes the PyTorch training loop, saves the model, and reports status to the master control via `RemoteLog`.

## 2. Architecture and Data Flow

```
 User ──stdin──►  Master Control (cmd)  ──ManagerLink──►  Master Control (back)
                       ▲                                        │
                       │ RemoteLog                              │ Training / DatabaseLink
                       │                                        ▼
   ┌───────────────────┴────────────────────┐   ┌─────────────────────────────┐
   ▼                                        ▼   ▼                             ▼
 Training Node                        Data Node  Training Node           Data Node
 (TrainPlugin)                      (DatabasePlugin) (SendToTrain)   (SendToDatabase)
   ▲                                        │
   └──────────── WithTrain ─────────────────┘
```

The end-to-end process of a training task:

1. The operator enters a command in the master control console (e.g. `apply <path>`).
2. The master control parses the operator's configuration file.
3. The master control sends `SendToTrain` (to create the task) to the training side, and `SendToDatabase` (to start sending data) to the data side.
4. The data node queries MySQL, preprocesses samples one by one, and streams tensors to the training side via `WithTrain.SendTrain`.
5. The training node buffers data (current task in memory, queued tasks written to disk under `DATA_PATH`), starts training — subject to the task's `TimeOut` limit — and saves the model upon receiving `Finish`.
6. The training node and the data node report status to the master control via `RemoteLog`; the master control writes these reports into `TRAINCONFIG_PATH/<task-id>.txt`.
7. The operator uses `checkLog <id> [start] [end]` to view logs locally.

## 3. Directory Structure

```
program/
├── Training/                 # Go module "train" — Master Control
│   ├── back/                 #   Business layer (ManagerLink server, task scheduling)
│   ├── cmd/                  #   Interaction layer (interactive CLI + RemoteLog server)
│   ├── config/               #   Configuration types and loading
│   ├── pkg/                  #   General tools (logs, time, command reading)
│   ├── manager/              #   Task/id management
│   ├── sender/               #   Training-side gRPC client
│   ├── Database/             #   Data-side gRPC client
│   ├── selector/             #   Data filtering criteria
│   ├── taskdb/               #   Task repository (bbolt)
│   └── commandLine/          #   Business-layer command handlers
├── DatabasePlugin/           # Go module "database" — Data Node
│   ├── main.go
│   ├── receive/              #   DatabaseLink gRPC server
│   ├── send/                 #   WithTrain gRPC client
│   ├── report/               #   RemoteLog gRPC client (reporting to master control)
│   ├── handler/              #   MySQL query + data preprocessing (image/text/json)
│   ├── controller/           #   Worker dispatch
│   ├── manager/              #   Task/id management
│   └── pkg/                  #   Logging helpers
└── TrainPlugin/              # Python — Training Node
    ├── main.py               #   Entry point (starts the two gRPC servers)
    ├── server/               #   Training + WithTrain gRPC servicers
    ├── task/                 #   Task manager (queue, data routing, file buffer, timeout)
    ├── train/                #   Training engine (prepare/train, logging)
    ├── data/                 #   Dataset/DataLoader, tensor decoding, file buffer
    ├── model/                #   Model load/save
    ├── client/               #   RemoteLog gRPC client
    ├── proto/                #   protobuf definitions
    └── proto_gen/            #   Generated gRPC code
```

## 4. Environment Variables

### 4.1 Data Node (`DatabasePlugin`)

| Variable   | Required | Default | Description            |
|------------|----------|---------|------------------------|
| `DATA_LOG` | No       | —       | Local log file path    |

### 4.2 Training Node (`TrainPlugin`)

| Variable                 | Required | Default             | Description                                                    |
|--------------------------|----------|---------------------|----------------------------------------------------------------|
| `TRAIN_LOG`              | Yes      | —                   | Training log directory (process exits if missing)              |
| `TRAIN_LISTEN`           | No       | `0.0.0.0:50052`     | Listen address of the `Training` service (used by master control) |
| `TRAIN_WITHTRAIN_LISTEN` | No       | `0.0.0.0:50054`     | Listen address of the `WithTrain` service (used by the data node) |
| `TRAIN_MAX_WORKERS`      | No       | `32`                | gRPC thread pool size                                          |
| `TRAIN_DATA_WAIT_TIMEOUT`| No       | `None`              | Seconds to wait for data (`None` = wait until `Finish`)        |
| `DATA_PATH`              | Yes      | `./train_data_buffer` | Directory for spooling queued-task data to disk              |

### 4.3 Master Control (`Training`)

| Variable           | Required | Default | Description                                                          |
|--------------------|----------|---------|----------------------------------------------------------------------|
| `TRAINCONFIG_PATH` | Yes      | —       | Log file directory (`<directory>/<task-id>.txt`)                     |
| `REMOTE_URL`       | No       | —       | Master control `RemoteLog` service address                           |
| `BACKPATH`         | No       | —       | Startup path used when the master control business layer is unreachable |

## 5. Ports and gRPC Services

| Port     | Service        | Server                | Client                | Methods                                                                                          |
|----------|----------------|-----------------------|-----------------------|---------------------------------------------------------------------------------------------------|
| `:50051` | `ManagerLink`  | Master Control (back) | Master Control (cmd)  | `CheckManager`, `ApplyManager`, `TaskManager`, `CancelManager`, `Exit`, `DeleteTaskDb`, `ViewTaskDb` |
| `:50051` | `DatabaseLink` | Data Node             | Master Control (back) | `SendToDatabase`, `Cancel`                                                                        |
| `:50052` | `Training`     | Training Node         | Master Control (back) | `SendToTrain`, `QueryTraining`, `CancelTraining`                                                  |
| `:50054` | `WithTrain`    | Training Node         | Data Node             | `Check`, `SendTrain`, `Finish`, `ReportError`                                                     |
| `:50053` | `RemoteLog`    | Master Control (cmd)  | Training Node / Data Node | `TrainRemote`, `DatabaseRemote`                                                               |

> The master control and the data node both listen on `50051`, but they should be deployed on different machines.

The master-control configuration maps the two training-side services to their addresses: `train_backend_url` → `Training` (`:50052`), `train_data_url` → `WithTrain` (`:50054`).

## 6. Master Control Command Line Usage

The command reads from standard input. Each parameter can be passed either by position or in the form `--key=value` (the `--` prefix and the `key=` part are removed).

| Command        | Grammar                       | Explanation                                                                |
|----------------|-------------------------------|----------------------------------------------------------------------------|
| `apply`        | `apply <path>`                | Apply the training template (configuration file) and submit the task       |
| `task`         | `task <all> <id>`             | Query task status; `<all>` is `true`/`false` (all tasks vs a single task id) |
| `cancel`       | `cancel <all> <id>`           | Cancel task(s)                                                             |
| `checkLog`     | `checkLog <id> [start] [end]` | Print the task log in `TRAINCONFIG_PATH/<id>.txt`                          |
| `viewtaskdb`   | `viewtaskdb <key> <time>`     | View a record in the task repository                                       |
| `deletetaskdb` | `deletetaskdb <key> <time>`   | Delete a record from the task repository                                   |
| `exit`         | `exit`                        | Exit the console (the **only** way for the process to terminate)           |

### `checkLog` Time Filter

The time parameter format is `YYYY-MM-DD-HH:MM:SS` (for example, `2026-08-20-10:00:00`).

- Only `start` given → display all logs after that time
- Only `end` given → display all logs before that time
- Both given → display logs within `[start, end]`
- Neither given → display all logs

Examples:

```bash
checkLog task-001
checkLog task-001 2026-08-20-00:00:00
checkLog task-001 2026-08-20-00:00:00 2026-08-20-23:59:59
```

### Task Time Limit (`TimeOut`)

The training template applied by `apply` may carry a `TimeOut` field (string) that
limits how long a task may occupy the pipeline. The format combines `d`/`h`/`m`/`s`
units freely, e.g. `90m` or `1h30m`. The timer starts when the task becomes the
current task and covers both data receiving and training. When it expires:

- the training node cancels the task, reports `cancelled` with message
  `<task-id>:timeout`, deletes the task's spool file (`DATA_PATH/<task-id>.txt`),
  and promotes the next queued task;
- the data node then receives `no matching training task` on its next
  `SendTrain`/`Finish`, so it stops dispatching/preprocessing that task's remaining
  data, removes the task from its id/task management, and disconnects.

## 7. Configuration File

The configuration applied by `apply <path>` is a **JSON file** whose fields map to
the Go struct `config.Config`.

### Top-level fields

| Field              | Type   | Description                                       |
|--------------------|--------|---------------------------------------------------|
| `name`             | string | Task/plan name                                    |
| `description`      | string | Description                                       |
| `refresh`          | int    | Status refresh interval                           |
| `train_backend_url`| string | Address of the training-side `Training` service (`:50052`) |
| `train_data_url`   | string | Address of the training-side `WithTrain` service (`:50054`) |
| `db`               | object | Database connection configuration                 |
| `dataset`          | object | Dataset configuration                             |
| `train_config`     | object | Training configuration                            |

### `db` fields

| Field      | Type   | Description       |
|------------|--------|-------------------|
| `DbName`   | string | Database type     |
| `Address`  | string | Database address  |
| `Port`     | int    | Database port     |
| `Account`  | string | Account           |
| `Password` | string | Password          |

### `dataset` fields

| Field             | Type   | Description                      |
|-------------------|--------|----------------------------------|
| `Input`           | string | Data type (txt / image / mix)    |
| `FilePath`        | string | Data storage path                |
| `Validation`      | int    | Validation ratio (%)             |
| `CategoriesNumber`| int    | Number of categories             |

### `train_config` fields

| Field             | Type   | Description                                          |
|-------------------|--------|------------------------------------------------------|
| `Epochs`          | int    | Number of epochs                                     |
| `LearningRate`    | float  | Learning rate                                        |
| `LossFunction`    | string | Loss function name                                   |
| `EarlyStop`       | bool   | Whether to enable early stopping                     |
| `EarlyStopPatient`| int    | Early-stop patience                                  |
| `ModelSave`       | string | Model save path                                      |
| `LogSave`         | string | Log file name                                        |
| `TimeOut`         | string | Training time limit (`d`/`h`/`m`/`s` combos, e.g. `1h30m`) |

> The data filtering criteria (`selector`) are not part of this JSON; they are
> provided separately via a criteria file (one `field operator value` per line).

### Configuration file example

```json
{
  "name": "image-classification",
  "description": "image classification training task",
  "refresh": 1,
  "train_backend_url": "127.0.0.1:50052",
  "train_data_url": "127.0.0.1:50054",
  "db": {
    "DbName": "mysql",
    "Address": "127.0.0.1",
    "Port": 3306,
    "Account": "root",
    "Password": "your_password"
  },
  "dataset": {
    "Input": "image",
    "FilePath": "/data/train",
    "Validation": 30,
    "CategoriesNumber": 10
  },
  "train_config": {
    "Epochs": 10,
    "LearningRate": 0.001,
    "LossFunction": "cross_entropy",
    "EarlyStop": true,
    "EarlyStopPatient": 5,
    "ModelSave": "/models/model.pth",
    "LogSave": "train.txt",
    "TimeOut": "1h30m"
  }
}
```

## 8. Optimizations & Concurrency Model

### 8.1 Concurrency Model

- **Data Node** (`DatabasePlugin`): the controller dispatches samples to multiple
  concurrent workers (each owning a `WithTrain` sender and a MySQL handler); an I/O
  semaphore (`maxConcurrentIO = 10`) caps the number of samples being read from
  disk at the same time, preventing disk saturation.
- **Training Node** (`TrainPlugin`): a single-task FIFO queue with a two-thread
  pipeline — a persistent *prepare* thread (wait for data, load the spool file,
  build the DataLoader) and a *train* thread. While one task is training, the next
  task is prepared concurrently.
- **Task time limit**: the `TimeOut` timer starts when a task is dequeued and
  covers both data receiving and training; on expiry the training node cancels the
  task (reported as `<task-id>:timeout`) and deletes its spool file, while the data
  node stops processing that task's data and cleans it up once it observes
  `no matching training task`.

### 8.2 Optimizations

- Image pipeline: decoded images are converted 1:1 to RGBA, a nearest-neighbor
  source-coordinate map is precomputed, and pixels are read directly from the RGBA
  `Pix` buffer — avoiding per-pixel `At`/`Set`/`RGBA` calls.
- Batch image preprocessing reuses one float32 buffer and one byte buffer across
  all images (N allocations → 1), removing the inner-buffer copy.
- JSON (line-per-sample) files are parsed with a streaming `bufio.Scanner`
  instead of reading the whole file into memory.
- Database pagination uses keyset pagination (`WHERE ID > ? ORDER BY ID LIMIT ?`)
  instead of `OFFSET`, keeping each page O(pageSize).
- Queued-task data is spooled to disk (numpy `.npy` blobs with length-prefixed
  records) and streamed back record-by-record when training starts, bounding
  memory usage.

## 9. systemd Daemon Deployment

The data node and the training node each ship a systemd unit + installation script:

- `databasedaemon/setup.sh` (+ `databaseplugin.service`)
- `traindaemon/setup.sh` (+ `trainplugin.service`)

Running `setup.sh` **automatically performs the installation**: it copies the
compiled binary into `/opt/`, copies the service unit into
`/etc/systemd/system/`, generates the environment-variable template, and runs
`systemctl enable` + start.

```bash
cd databasedaemon && sudo bash setup.sh   # Data Node
cd traindaemon && sudo bash setup.sh       # Training Node
```

Installation notes:

- **Binary location**: the compiled binary `LDP` (data node) / `LTP` (training
  node) must sit alongside `setup.sh`; the script copies it with `cp -f` to
  `/opt/databaseplugin` / `/opt/trainplugin`.
- **Environment-variable file location**: the Data Node uses
  `/etc/default/databaseplugin`, the Training Node uses
  `/etc/default/trainplugin`. After installation, edit these files to set the
  environment variables (e.g. `DATA_LOG`, `TRAIN_LOG`, `DATA_PATH`), then run
  `systemctl restart <service-name>` to apply.
- **Overwrite note**: if a file of the same name already exists at the target
  path, `setup.sh` overwrites it (both the binary and the service unit are copied
  with `cp -f`).
- **Management commands**: `systemctl status|restart|stop <service-name>`; view
  logs with `journalctl -u <service-name> -f`.
