# Kamaji 

## Overview

The Kamaji is a command-line tool designed to manage and execute  targets defined in a YAML configuration file. It is influenced by Bazel, particularly in its ability to be extended using Python templates. Kamaji provides options for adding additional support for third-party dependencies to finally pass necessary configuration or authentication to a target binary or a script.

Best example case is using Terraform as a target and passing credentials and configuration to the Terraform binary.

Additionaly, Kamaji provides a mechanism to download third-party dependencies and cache them, so that the same third-party dependencies can be managed across different targets and do not need to be downloaded again. Also, by specifying dependency sha256, Kamaji can ensure the integrity of the downloaded third-party dependencies.

## Features

- **Third-party Initialization**: Initializes third-party dependencies required by the build target.
- **Python Extension**: Allows for extension and customization using Python templates, enabling flexible build configurations.
- **Dependency Caching**: Caches third-party dependencies and ensures their integrity using sha256 checksums.

## Usage

```bash
kamaji <target>
```

When you run `kamaji <target>`, it will execute the target with the given name. kamaji will look for a `BUILD.yaml` file in the current directory and execute the target specified in the `BUILD.yaml` file. BUILD.yaml file location can be specified using the `--build` flag.

```bash
kamaji <target> --build <build_file>
```
Additionaly, kamaji provides a mechanism to pass additional arguments to the target binary or a script.

```bash
kamaji <target> -- <additional_arguments>
```





