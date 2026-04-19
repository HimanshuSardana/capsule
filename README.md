# Capsule

Capsule is a simple containerization tool designed to run Linux container
images in isolated environments using namespaces (PID, UTS, and mount
namespaces). The tool allows users to download container images, extract them,
and run them in a separate namespace with minimal setup.

## Features

* **List available images**: View the list of available container images defined in `images.json`.
* **Run container**: Run a container image in a specified directory, with its own isolated environment using Linux namespaces (PID, UTS, Mount).
* **Minimal setup**: The container runs with basic Linux tools (`busybox` shell, DNS setup).
* **Support for `tar.gz` image format**: Download and extract `tar.gz` image files into the container root filesystem.

## Usage

### Commands

* `list`: Lists all available container images from `images.json`.
* `run <image> <dir>`: Runs the specified image in the specified directory.

### Example

```bash
capsule list

capsule run base testdir2
```

### Container Setup

* **Namespace isolation**:
  The container is started with isolated namespaces (PID, UTS, mount).

* **Basic mounts**:
  * `/proc` is mounted for process information.
  * `/dev/pts` is mounted for controlling terminal devices.

* **Shell execution**:
  Once the container starts, it runs a simple shell (`/bin/busybox sh`).

## Installation

1. Clone this repository:

   ```bash
   git clone https://github.com/yourusername/capsule.git
   cd capsule
   ```

2. Build the Go binary:

   ```bash
   go build -o capsule .
   ```

## Configuration

The `images.json` file defines the available container images. The file must be located in the same directory as the executable. It should be formatted like this:

```json
[
  {
    "name": "base",
    "url": "https://dl-cdn.alpinelinux.org/alpine/v3.23/releases/x86_64/alpine-minirootfs-3.23.4-x86_64.tar.gz"
  }
]
```
