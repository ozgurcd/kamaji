#!/usr/bin/env python3
"""
Terraform Runner Template
"""

import logging
import argparse
import json
import sys
import os
import subprocess

#from .. import TerraformRunner


NO_INIT_COMMANDS=[
    "fmt", 
    "version",
    "validate", 
    "force-unlock", 
    "taint", 
    "untaint", 
    "show",
    "version"
]

class OnePasswordRunner:
    def __init__(self, deployment, op_tfvars_file, org_domain):
        logging.debug("Initializing OnePasswordRunner")
        self.op_cli_path = ""
        self.op_account = ""
        self.op_item_id = ""
        self.op_item_notes = ""
        self.deployment = deployment
        self.op_tfvars_file = op_tfvars_file
        self.org_domain = org_domain
        self.should_use_op = True

        logging.debug("Getting OP CLI Path")
        try:
            self.op_cli_path = subprocess.check_output(["which", "op"]).decode("utf-8")
        except subprocess.CalledProcessError as e:
            self.should_use_op = False
            logging.debug("Could not find 1Password executable, skipping")
            return 
        
        if self.op_cli_path.endswith("\n"):
            self.op_cli_path = self.op_cli_path.strip()
        
        logging.debug("OP CLI Path: %s" % self.op_cli_path)
        logging.debug("Getting OP Account")
        self.setAccount()
        logging.debug("OP Account: %s" % self.op_account)

    def shouldUseOP(self):
        return self.should_use_op
    
    def setAccount(self):
        command = f"{self.op_cli_path} account list | grep -E '[[:<:]]{ORG_DOMAIN}[[:>:]][[:space:]]' | awk '{{print $NF}}'"
        logging.debug("command: [%s]\n" % command)

        output = subprocess.check_output(['/bin/bash', '-o', 'pipefail', '-c', command], text=True).strip()
        if not output:
            logging.error("Can't reach 1Password account associated with your company email\n")
            self.should_use_op = False
            return

        self.op_account = output
    
    def getEnvVariables(self):
        op_envs = {}
        # if no tag found it fails.
        command = f"{self.op_cli_path} item list --account={self.op_account} --tags=terraform/{self.deployment} | grep -E '[[:<:]]{self.op_tfvars_file}[[:>:]][[:space:]]' | awk -F' ' '{{print $1}}'"
        logging.debug("command: [%s]\n" % command)

        op_item_id = subprocess.check_output(['/bin/bash', '-o', 'pipefail', '-c', command], text=True).strip()
        if not op_item_id:
            logging.error("Can't find 1Password item with terraform secrets\n")
            return op_envs
        
        logging.debug("op_item_id: %s" % op_item_id)

        command = f"{self.op_cli_path} item get {op_item_id} --account={self.op_account} --fields=notesPlain --format json | jq .value"
        logging.debug("command: [%s]\n" % command)

        op_item_notes = subprocess.check_output(['/bin/bash', '-o', 'pipefail', '-c', command], text=True).strip()
        if not op_item_notes:
            logging.error("Can't find 1Password item with terraform secrets\n")
            return op_envs

        op_item_notes = op_item_notes.encode().decode('unicode_escape')
        op_item_notes = op_item_notes.strip('"')
        variable_lines = op_item_notes.strip().split('\n')    
        #logging.debug("op_item_notes: %s" % op_item_notes)

        for line in variable_lines:
            if line and line[0] == '#':
                # Skipping comments
                continue

            if '=' in line:
                # Splitting each line into the variable name and its value
                var_name, var_value = line.split('=', 1)
                var_name = var_name.encode().decode('unicode_escape').strip().strip('\'"') 
                var_value = var_value.encode().decode('unicode_escape').strip().strip('\'"') 
                # Setting the environment variable
                logging.debug("Setting env var: %s=%s" % (var_name, var_value))
                op_envs[var_name] = var_value

        return op_envs

# class TerraformRunner:
#     def __init__(self, terraform_path, working_dir, terraform_backend_config, terraform_debug, awsProfile, awsRegion, kubeconfig):        
#         logging.debug("Initializing TerraformRunner")
#         self.terraform_path = terraform_path[0]
#         self.working_dir = working_dir
#         self.tfvarfilepath = ''
#         self.aws_profile = awsProfile
#         self.aws_region = awsRegion
#         self.kubeconfig = kubeconfig
#         self.current_workspace = ""
#         self.terraform_backend_config = terraform_backend_config
#         self.terraform_debug = terraform_debug
#         self.op_envs = [] # needs to be filled by setEnvVariables function

#         logging.debug("Setting Terraform Path: %s" % self.terraform_path)
#         logging.debug("Setting Terraform WD: %s" % self.working_dir)
#         logging.debug("Setting Terraform AWS Profile: %s" % self.aws_profile)
#         logging.debug("Setting Terraform AWS Region: %s" % self.aws_region)
#         logging.debug("Setting Terraform terraform_backend_config: %s" % self.terraform_backend_config)

#     def is_there_any_uncommitted_changes(self):
#         build_workspace_dir = os.getenv("BUILD_WORKSPACE_DIRECTORY")
#         if not build_workspace_dir:
#             print("Error: BUILD_WORKSPACE_DIRECTORY is not set")
#             return False

#         result = subprocess.run(["git", "rev-parse", "--show-toplevel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    
#         if result.returncode != 0:
#             print(f"Error finding the top-level Git directory: {result.stderr.decode('utf-8')}")
#             return False
    
#         env = os.environ.copy()
#         env["GIT_DIR"] = os.path.join(build_workspace_dir, ".git")
#         env["GIT_WORK_TREE"] = build_workspace_dir
        
#         result = subprocess.run(
#             ["git", "status", "--porcelain"],
#             stdout=subprocess.PIPE,
#             stderr=subprocess.PIPE,
#             env=env,
#             cwd=build_workspace_dir
#         )       

#         if result.stderr:
#             print(f"Git error: {result.stderr.decode('utf-8')}")
#             return False 
        
#         if result.stdout:
#             logging.debug("There are uncommitted changes in the repository")
#             logging.debug("Uncommitted changes: %s", result.stdout.decode())
#             return True
        
#         return False

#     def setEnvVariables(self, envs):
#         self.op_envs = envs

#     def init_with_backend(self, be_config):
#         iargs=[
#             "-backend=true", 
#             "-reconfigure"
#         ] 
#         iargs.append("-backend-config=bucket=%s" % be_config['bucket'])
#         iargs.append("-backend-config=key=%s" % be_config['key'])
#         iargs.append("-backend-config=dynamodb_table=%s" % be_config['dynamodb_table'])
#         iargs.append("-backend-config=region=%s" % be_config['region'])
        
#         return self.run("init", iargs)

#     def init_without_backend(self):
#         logging.debug("Initializing Terraform without backend config")
#         return self.run("init", ["-backend=false"])

#     def init(self):
#         logging.info("Initializing Terraform")
#         logging.debug("Terraform Backend Config: %s" % self.terraform_backend_config)
#         if self.terraform_backend_config:
#             be_config = json.loads(self.terraform_backend_config)
#             logging.debug("Backend Config: %s" % be_config)
#         else:
#             logging.debug("No Backend Config")
#             be_config = "{}"
        
#         if be_config != "{}":
#             self.init_with_backend(be_config)
#         else:
#             self.init_without_backend()

#     def setRuntimeVarsFile(self, tfvarfilepath):
#         logging.debug("Setting Runtime Vars File: %s" % tfvarfilepath)
#         self.tfvarfilepath = tfvarfilepath

#     def return_to_default_workspace(self):
#         self.switch_workspace("default")

#     def switch_workspace(self, workspace):
#         logging.debug("Switching to workspace: %s", workspace)

#         if not self.run("workspace", ["select", workspace]):
#             self.run("workspace", ["new", workspace])

#     def run(self, subcmd, args):
#         caws_region = os.environ.get("AWS_REGION", "")
#         if self.aws_region == "":
#             logging.error("AWS Region is not set")
#             return False
        
#         logging.debug("Current AWS Region: %s", caws_region)
#         logging.debug("Req Terraform AWS Region: %s", self.aws_region)

#         if caws_region != self.aws_region:
#             logging.debug("Setting AWS Region to: %s", self.aws_region)
#             os.environ["AWS_REGION"] = self.aws_region

#         logging.debug("Executing terraform command: %s", subcmd)
#         logging.debug("Executing terraform args: %s", args)

#         spcmd = [self.terraform_path, subcmd]
#         logging.debug("spcmd: %s", spcmd)

#         if subcmd not in NO_REGION_COMMANDS:
#             spcmd.append("-var=aws_region=%s" % self.aws_region)
#         logging.debug("spcmd: %s", spcmd)

#         if subcmd not in NO_INPUT_COMMANDS:
#             logging.debug("%s command detected, adding -input=false", subcmd)
#             spcmd.append("-input=false")

#         if subcmd not in NO_ARG_COMMANDS and self.tfvarfilepath:
#             spcmd.extend(["-var-file", self.tfvarfilepath])

#         spcmd.extend(args)

#         orig_envs = os.environ.copy()

#         envs = {}
#         #envs['PATH'] = orig_envs['PATH']
#         # set the environment variables from 1Password
#         if len(self.op_envs) > 0:
#             for key, value in self.op_envs.items():
#                 if value is not None:  # Ensure value is not None
#                     envs[key] = value

#         envs['AWS_PROFILE'] = self.aws_profile or ""  # Ensure default empty string
#         envs['KUBECONFIG'] = self.kubeconfig or ""    # Ensure default empty string
#         envs['KUBE_CONFIG_PATH'] = self.kubeconfig or ""  # Ensure default empty string

#         if self.aws_region:
#             envs['AWS_REGION'] = self.aws_region

#         logging.debug("Executing command in : %s", self.working_dir)
#         logging.debug("Executing Terraform with env variables: %s", envs)
#         logging.debug("Executing command: %s", spcmd)
#         try:
#             proc = subprocess.Popen(
#                 spcmd,
#                 cwd = self.working_dir,
#                 stderr = subprocess.PIPE,
#                 stdin=sys.stdin,
#                 env=envs
#             )
#             _, stderr = proc.communicate()

#             if self.terraform_debug == "true":
#                 logging.debug("Terraform stderr: %s", stderr.decode())
            
#             if proc.returncode != 0:
#                 error_message = stderr.decode()
#                 logging.error("Terraform command failed: %s, Error: [%s]", subcmd, error_message)
#                 return False
#             else:
#                 return True
        
#         except KeyboardInterrupt:
#             pass
#         finally:
#             if caws_region != self.aws_region:
#                 logging.debug("Unsetting AWS_REGION")
#                 if 'AWS_REGION' in os.environ:
#                     del os.environ['AWS_REGION']
    
##########################
logger = logging.getLogger()

def setupLogging(log_verbosity):
    logger.setLevel(getattr(logging, log_verbosity))
    handler = logging.StreamHandler(sys.stdout)
    formatter = logging.Formatter(
        fmt = '[%(levelname)s] (%(asctime)s): %(message)s',
        datefmt = '%m/%d/%Y %I:%M:%S %p')
    handler.setFormatter(formatter)
    logger.addHandler(handler)

def _main():
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument('--terraform_executable')
    parser.add_argument('--log_verbosity')
    parser.add_argument('--aws_profile')
    parser.add_argument('--kubeconfig')
    parser.add_argument('--op_tfvars_file')
    parser.add_argument('--runtime_vars_file')
    parser.add_argument('--terraform_workspace')
    parser.add_argument('--terraform_backend_config')
    parser.add_argument('--aws_region')
    parser.add_argument('--terraform_debug')

    flags, args = parser.parse_known_args()
    setupLogging(flags.log_verbosity)

    logging.info("log_verbosity: %s" % flags.log_verbosity)

    if flags.aws_region == "":
        logging.error("AWS Region is not set")
        return 1

    workspace_root = os.environ.get("BUILD_WORKSPACE_DIRECTORY")

    ORG_DOMAIN = os.environ.get("KAMAJI_ORGANIZATION_DOMAIN")
    PYTHONPATH = os.environ.get("PYTHONPATH")

    base_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    common_dir = os.path.join(base_dir, "common")
    sys.path.append(common_dir)
    from terraform import TerraformRunner

 
    #BASE_DIR = os.environ.get("KAMAJI_BASE_DIR")
    # try:
    #     with open(config_path, 'r') as config_file:
    #         config = json.load(config_file)
    #     logging.debug("Configuration loaded successfully.")
    # except FileNotFoundError:
    #     logging.error(f"Error: The file '{config_path}' was not found.")
    #     os.exit(1)
    # except json.JSONDecodeError as e:
    #     logging.error(f"Error: Failed to decode JSON. Details: {e}")
    #     os.exit(1)
    # except Exception as e:
    #     logging.error(f"An unexpected error occurred: {e}")
    #     os.exit(1)

    # BASE_DIR = config.get("BASE_DIR")
    # ORG_DOMAIN = config.get("ORG_DOMAIN")

    # if not BASE_DIR:
    #     logging.error("NO BASE_DIR set")
    #     os.exit(1)
    # else:
    #     logging.debug("BASE_DIR: %s" % BASE_DIR)

    if not ORG_DOMAIN:
        logging.error("Can't find KAMAJI_ORGANIZATION_DOMAIN environment variable")
        os.exit(1)
    else:
        logging.debug("ORG_DOMAIN: %s" % ORG_DOMAIN)

    logging.debug("terraform_workspace: %s " % flags.terraform_workspace)
    logging.debug("We got aws profile: %s " % flags.aws_profile)
    logging.debug("We got terraform exec: %s " % flags.terraform_executable)
    logging.debug("We got op_tfvars_file: %s " % flags.op_tfvars_file)
    logging.debug("We got aws region: %s " % flags.aws_region)
    logging.debug("Terraform debug: %s " % flags.terraform_debug)

    if flags.terraform_backend_config:
        logging.debug("We got terraform backend config: " + flags.terraform_backend_config)

    build_working_dir = os.getcwd()
    build_workspace_dir = workspace_root
    logging.debug("BUILD_WORKING_DIRECTORY: %s" % build_working_dir)
    logging.debug("BUILD_WORKSPACE_DIRECTORY: %s" % build_workspace_dir)

    if build_working_dir == None:
        logging.error("BUILD_WORKING_DIRECTORY is not set")
        return 1
    
    if build_workspace_dir == None:
        logging.error("BUILD_WORKSPACE_DIRECTORY is not set")
        return 1
    
    if build_working_dir.startswith(build_workspace_dir):
        remaining_path = build_working_dir[len(build_workspace_dir):]
    else:
        logging.error("BUILD_WORKING_DIRECTORY is not a subdirectory of BUILD_WORKSPACE_DIRECTORY")
        logging.error("BUILD_WORKING_DIRECTORY: %s" % build_working_dir)
        logging.error("BUILD_WORKSPACE_DIRECTORY: %s" % build_workspace_dir)
        return 1
    
    # if not remaining_path.startswith(BASE_DIR):
    #     logging.error("BUILD_WORKING_DIRECTORY is not a subdirectory of %s" % BASE_DIR)
    #     return 1
    # else:
    #     actual_path = remaining_path[len(BASE_DIR):]

    # if actual_path.startswith("/"):
    #     actual_path = actual_path[1:]
    # logging.debug("Actual Path: %s" % actual_path)

    terraform_subcmd = ''
    runtimevarsfilepath=''

    if flags.runtime_vars_file:
        runtimevarsfilepath= os.path.join(build_working_dir, flags.runtime_vars_file)   
    
    logging.debug("runtimevarsfilepath: %s", runtimevarsfilepath)
    logging.debug("Verbosity: %s" % flags.log_verbosity)
    logging.debug("Args: %s " % args)

    terraform_executable = [flags.terraform_executable]   
    terraform_executable = [os.path.join(os.getcwd(), flags.terraform_executable)]
    tr = TerraformRunner(
        terraform_executable, 
        build_working_dir,
        flags.terraform_backend_config,
        flags.terraform_debug,
        awsProfile=flags.aws_profile,
        awsRegion=flags.aws_region,
        kubeconfig=flags.kubeconfig)

    logging.debug("Terraform Subcommand: [%s]" % terraform_subcmd)
    if len(args) == 0:
        terraform_subcmd = "plan"
    else:
        terraform_subcmd = args[0]
        args.pop(0)

    if terraform_subcmd not in NO_INIT_COMMANDS:
        logging.debug("Command [%s] requires terraform init" % terraform_subcmd)
        tr.init()
    else:
        logging.debug("Command [%s] does not require terraform init" % terraform_subcmd)

    tr.switch_workspace(flags.terraform_workspace)

    if flags.op_tfvars_file:
        o = OnePasswordRunner(
            flags.terraform_workspace, 
            flags.op_tfvars_file.strip(),
            ORG_DOMAIN
        )
        
        if o.shouldUseOP():
            logging.debug("Using 1Password")
            op_envs = o.getEnvVariables()
            tr.setEnvVariables(op_envs)
        else:
            logging.debug("Not using 1Password")
    
    if flags.runtime_vars_file:
        tr.setRuntimeVarsFile(runtimevarsfilepath)

    logging.debug("Terraform Subcommand: %s" % terraform_subcmd)
    logging.debug("Terraform args: %s" % args)
    
    bypass_git = os.environ.get("BYPASS_GIT", "false")
    if bypass_git != "true" and tr.is_there_any_uncommitted_changes():
        logging.error("There are uncommitted changes in the repository, commit them before running terraform")
        return 1
    
    exitCode = 0
    if not tr.run(terraform_subcmd, args):
        tr.return_to_default_workspace()
        exitCode = 1
    else:
        tr.return_to_default_workspace()

    sys.exit(exitCode)
    tr.return_to_default_workspace()

## Main
_main()
