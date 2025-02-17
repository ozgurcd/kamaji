# Getting Started with Kamaji

This guide helps you set up Kamaji for Terraform targets, explaining key configuration files and how to create new custom rules.

---

## 1. `kamaji.workspace.yaml`

This file describes your global workspace config including:
- Where your rules are stored (`rules_directory`)
- Shared variables (`workspace_vars`) for all targets, if you need to pass variables to the targets.
- References to third-party dependencies like Terraform or kubectl. Please note that kamaji won't use the tools installed on your OS. Instead, it will download the necessary tools from the URLs provided in the `third_party` section and verify the SHA256 checksum. That way you can have the same version of tools across different machines without having to install them manually. Also, this approach you to guarantee of the integrity and authenticity of the tools.

**Example**:
```yaml
---
rules_directory: "//kamaji/rules"
workspace_vars:
  - org_domain: "example.com"
    base_dir: "/projects/infra"
third_party:
  - name: terraform_1_9_0
    ...
```

Place this file at your workspace root, aka the top level directory that kamaji can access. Once you define a third party tool, you can use it in your targets by referencing its name following the `@@` syntax.

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

Within a Terraform directory, create a `BUILD.yaml` defining the targets (such as “staging”, “production”):
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
- **config**: Provides any needed parameters (e.g., which Terraform version to use, region, workspace name, etc.). Definition of this config is up to the Python script and defined in the rule directory, in variables.yaml file.

This config is passed to the Python script as a dictionary. If the given config does not match the expected config, the Python script will raise an error.

---

## 3. Rules Directory Overview

Kamaji looks in the `rules` directory for Python scripts to execute. Example structure:
```
├─ rules
│   └─ run_terraform
│       └─ run_terraform.py
│       └─ variables.yaml
├─ common
│   └─ terraform.py
```

- **run_terraform.py**: The entry script run by Kamaji. It typically imports shared logic from `terraform.py`.
- **terraform.py**: A helper module (in `common`) that manages command construction, environment setup, etc.
- **variables.yaml**: A file that defines the expected config for the rule. It also includes the default values for the config.

There is also a "common" directory that contains shared logic for all rules. Since there is a file in common directory with the name "terraform.py" which defines the shared logic for all terraform rules when Kamaji runs a target referencing `run_terraform/run_terraform.py`, it loads `terraform.py` internally to handle Terraform CLI commands.

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
    # build a Terraform command and run it
    cmd = ["terraform"] + args
    # add any logic or environment variables needed...
```

---

## 5. Creating New Rules

To add a new rule:
1. Create a new directory under `rules` with a Python file (e.g., `rules/new_rule/my_rule.py`).
2. Write a `main()` function that uses your shared logic from `common`, or from libraries you install.
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

## 6. // notation

The `//` notation is used to reference the root of the workspace. It is equivalent to the workspace root directory, aka the directory where the `kamaji.workspace.yaml` file is located.

**Example**:
```yaml
---
targets:
  - name: "my_custom_target"
    rule: "//rules/new_rule/my_rule.py"
    config:
      some_key: "some_value"
--- 

