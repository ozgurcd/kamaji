# Getting Started with Kamaji

This guide explains how to set up Kamaji for Terraform targets, covering key configuration files and instructions on creating custom rules.

---

## 1. `kamaji.workspace.yaml`

This file describes your global workspace configuration, including:
- **rules_directory**: The location where your rules are stored.
- **workspace_vars**: Shared variables for all targets (useful for passing variables to targets).
- **third_party**: References to third-party dependencies such as Terraform or kubectl. Please note that Kamaji won't use the tools installed on your OS. Instead, it downloads the necessary tools from the URLs provided in the `third_party` section and verifies their SHA256 checksums. This approach ensures that you have the same version of tools across different machines while also guaranteeing their integrity and authenticity.

**Example**:
```yaml
---
rules_directory: "//rules"
workspace_vars:
  - org_domain: "example.com"
    base_dir: "/projects/infra"
third_party:
  - name: terraform_1_9_0
    ...
```

Place this file in your workspace root (the top-level directory that Kamaji can access). Once you define a third-party tool, you can reference its name in your targets using the `@@` syntax. Please note that Kamaji will attempt to download the version that matches the OS and architecture of the machine on which it is running.

**Example**:
```yaml
---
targets:
  - name: "staging"
    rule: "run_terraform/run_terraform.py"
    config:
      terraform_executable: "@@terraform_1_10_5"
---
```

## 2. `BUILD.yaml` for Terraform

Within a Terraform directory, create a `BUILD.yaml` file that defines the targets (such as "staging" or "production"):
```yaml
---
targets:
  - name: "staging"
    rule: "run_terraform/run_terraform.py"
    config:
      terraform_executable: "@@terraform_1_10_5"
      terraform_workspace: "staging"
      ...
```
- **rule**: Points to the Python script that handles Terraform tasks (`run_terraform.py`).
- **config**: Provides any necessary parameters (e.g., which Terraform version to use, region, workspace name, etc.). The definition of this configuration is determined by the Python script and is specified in the rule directory's `rule_definition.yaml` file.

This configuration is passed to the Python script as a dictionary. If the provided configuration does not match the expected format, the Python script will raise an error.

---

## 3. Rules Directory Overview

Kamaji searches the `rules` directory for Python scripts to execute. An example structure:
```
├─ rules
│   └─ run_terraform
│       ├─ run_terraform.py
│       └─ rule_definition.yaml
├─ common
│   └─ terraform.py
```

- **run_terraform.py**: The entry script run by Kamaji. It typically imports shared logic from `terraform.py`.
- **terraform.py**: A helper module (located in the `common` directory) that manages command construction, environment setup, etc.
- **rule_definition.yaml**: A file that defines the expected configuration for the rule, including default values.

In addition, there is a `common` directory that contains shared logic for all rules. When Kamaji runs a target referencing `run_terraform/run_terraform.py`, it automatically loads the `terraform.py` file from the `common` directory to handle Terraform CLI commands.

---

## 4. Example: `run_terraform.py` and `terraform.py`

**run_terraform.py** (entry point):
```python
# run_terraform.py
import sys
import terraform

def main():
    args = sys.argv[1:]
    terraform.run_terraform(args)

if __name__ == "__main__":
    main()
```

**terraform.py** (common helpers):
```python
# terraform.py
def run_terraform(args):
    # Build a Terraform command and run it
    cmd = ["terraform"] + args
    # Add any additional logic or environment variables as needed...
```

---

## 5. Creating New Rules

To add a new rule:
1. Create a new directory under `rules` containing a Python file (e.g., `rules/new_rule/my_rule.py`).
2. Write a `main()` function that utilizes the shared logic available in the `common` directory or from other installed libraries.
3. In your `BUILD.yaml`, reference `"new_rule/my_rule.py"` under `rule`.

Example minimal new rule:
```python
# my_rule.py
def main():
    print("Running a custom rule")

if __name__ == "__main__":
    main()
```

Update your `BUILD.yaml` accordingly:
```yaml
targets:
  - name: "my_custom_target"
    rule: "new_rule/my_rule.py"
    config:
      some_key: "some_value"
```

You can now run:
```
kamaji my_custom_target
```

---

## 6. `//` Notation

The `//` notation is used to reference the root of the workspace. It is equivalent to the workspace root directory, i.e., the directory where the `kamaji.workspace.yaml` file is located.

**Example**:
```yaml
---
targets:
  - name: "my_custom_target"
    rule: "//rules/new_rule/my_rule.py"
    config:
      some_key: "some_value"
---
```

