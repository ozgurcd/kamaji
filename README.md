# Kamaji 

## Overview

Kamaji is a command-line tool designed to manage and execute targets defined in a YAML configuration file. Inspired by Bazel, Kamaji simplifies the execution of commands by providing a wrapper that handles configuration, authentication, and dependency management.

Kamaji is particularly useful for DevOps and infrastructure automation tasks, as it can integrate seamlessly with tools like Terraform, kubectl, helm and other CLI-based applications. It ensures consistent configuration across different targets while allowing customization using Python templates.

Best example case is using Terraform as a target and passing credentials and configuration to the Terraform binary since it allows using different versions of Terraform for each directory in a Terraform configuration, removing the burden of updating all existing Terraform code when upgrading the version of Terraform.

## Why Use Kamaji?

1. **Simplified Execution**: Define reusable targets in a BUILD.yaml file and execute them without manually managing dependencies and configurations.
2. **Configuration Management**: Inject authentication credentials and environment configurations automatically into CLI tools.
3. **Python Extension Support**: Extend the functionality of Kamaji using Python templates for more flexible configurations.
4. **Dependency Caching**: Caches third-party dependencies and ensures their integrity using sha256 checksums.
5. **Secure and Consistent Execution**: Ensures that every execution runs in a separate directory in controlled environment with predefined configurations.
6. **Automation-Friendly**: Useful for CI/CD pipelines and developer workflows that require consistent, reproducible command executions.


## Features

- **Third-party Initialization**: Initializes third-party dependencies required by the build target.
- **Python Extension**: Allows for extension and customization using Python templates, enabling flexible build configurations.
- **Dependency Caching**: Caches third-party dependencies and ensures their integrity using sha256 checksums.
- **Authentication and Configuration Injection**: Passes environment variables, authentication credentials and command line arguments to targets, ensuring secure execution of commands.

## Usage

``bash
kamaji <target>
``

When you run `kamaji <target>`, it will execute the target with the given name. kamaji will look for a `BUILD.yaml` file in the current directory and execute the target specified in the `BUILD.yaml` file. BUILD.yaml file location can be specified using the `--build` flag.

``bash
kamaji <target> --build <build_file>
``
Additionaly, kamaji provides a mechanism to pass additional arguments to the target binary or a script.

``bash
kamaji <target> -- <additional_arguments>
``
This will translate to executing the target command with the specified arguments.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request.

## License

Kamaji is released under the MIT License.


