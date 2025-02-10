import logging
import os
import subprocess
import sys
import json

NO_ARG_COMMANDS = [
    "fmt",
    "version",
    "state",
    "workspace",
    "output",
    "force-unlock",
    "show",
    "init"
]

NO_INPUT_COMMANDS = [
    "show",
    "output",
    "version",
    "workspace",
    "force-unlock",
    "state"
]

NO_REGION_COMMANDS = [
    "workspace",
    "force-unlock",
    "state",
    "output",
]

class TerraformRunner:
    def __init__(self, terraform_path, working_dir, terraform_backend_config, terraform_debug, awsProfile, awsRegion, kubeconfig):        
        logging.debug("Initializing TerraformRunner")
        self.terraform_path = terraform_path[0]
        self.working_dir = working_dir
        self.tfvarfilepath = ''
        self.aws_profile = awsProfile
        self.aws_region = awsRegion
        self.kubeconfig = kubeconfig
        self.current_workspace = ""
        self.terraform_backend_config = terraform_backend_config
        self.terraform_debug = terraform_debug
        self.op_envs = [] # needs to be filled by setEnvVariables function

        logging.debug("Setting Terraform Path: %s" % self.terraform_path)
        logging.debug("Setting Terraform WD: %s" % self.working_dir)
        logging.debug("Setting Terraform AWS Profile: %s" % self.aws_profile)
        logging.debug("Setting Terraform AWS Region: %s" % self.aws_region)
        logging.debug("Setting Terraform terraform_backend_config: %s" % self.terraform_backend_config)

    def is_there_any_uncommitted_changes(self):
        build_workspace_dir = os.getenv("BUILD_WORKSPACE_DIRECTORY")
        if not build_workspace_dir:
            print("Error: BUILD_WORKSPACE_DIRECTORY is not set")
            return False

        result = subprocess.run(["git", "rev-parse", "--show-toplevel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    
        if result.returncode != 0:
            print(f"Error finding the top-level Git directory: {result.stderr.decode('utf-8')}")
            return False
    
        env = os.environ.copy()
        env["GIT_DIR"] = os.path.join(build_workspace_dir, ".git")
        env["GIT_WORK_TREE"] = build_workspace_dir
        
        result = subprocess.run(
            ["git", "status", "--porcelain"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
            cwd=build_workspace_dir
        )       

        if result.stderr:
            print(f"Git error: {result.stderr.decode('utf-8')}")
            return False 
        
        if result.stdout:
            logging.debug("There are uncommitted changes in the repository")
            logging.debug("Uncommitted changes: %s", result.stdout.decode())
            return True
        
        return False

    def setEnvVariables(self, envs):
        self.op_envs = envs

    def init_with_backend(self, be_config):
        iargs=[
            "-backend=true", 
            "-reconfigure"
        ] 
        iargs.append("-backend-config=bucket=%s" % be_config['bucket'])
        iargs.append("-backend-config=key=%s" % be_config['key'])
        iargs.append("-backend-config=dynamodb_table=%s" % be_config['dynamodb_table'])
        iargs.append("-backend-config=region=%s" % be_config['region'])
        
        return self.run("init", iargs)

    def init_without_backend(self):
        logging.debug("Initializing Terraform without backend config")
        return self.run("init", ["-backend=false"])

    def init(self):
        logging.info("Initializing Terraform")
        logging.debug("Terraform Backend Config: %s" % self.terraform_backend_config)
        if self.terraform_backend_config:
            be_config = json.loads(self.terraform_backend_config)
            logging.debug("Backend Config: %s" % be_config)
        else:
            logging.debug("No Backend Config")
            be_config = "{}"
        
        if be_config != "{}":
            self.init_with_backend(be_config)
        else:
            self.init_without_backend()

    def setRuntimeVarsFile(self, tfvarfilepath):
        logging.debug("Setting Runtime Vars File: %s" % tfvarfilepath)
        self.tfvarfilepath = tfvarfilepath

    def return_to_default_workspace(self):
        self.switch_workspace("default")

    def switch_workspace(self, workspace):
        logging.debug("Switching to workspace: %s", workspace)

        if not self.run("workspace", ["select", workspace]):
            self.run("workspace", ["new", workspace])

    def run(self, subcmd, args):
        caws_region = os.environ.get("AWS_REGION", "")
        if self.aws_region == "":
            logging.error("AWS Region is not set")
            return False
        
        logging.debug("Current AWS Region: %s", caws_region)
        logging.debug("Req Terraform AWS Region: %s", self.aws_region)

        if caws_region != self.aws_region:
            logging.debug("Setting AWS Region to: %s", self.aws_region)
            os.environ["AWS_REGION"] = self.aws_region

        logging.debug("Executing terraform command: %s", subcmd)
        logging.debug("Executing terraform args: %s", args)

        spcmd = [self.terraform_path, subcmd]
        logging.debug("spcmd: %s", spcmd)

        if subcmd not in NO_REGION_COMMANDS:
            spcmd.append("-var=aws_region=%s" % self.aws_region)
        logging.debug("spcmd: %s", spcmd)

        if subcmd not in NO_INPUT_COMMANDS:
            logging.debug("%s command detected, adding -input=false", subcmd)
            spcmd.append("-input=false")

        if subcmd not in NO_ARG_COMMANDS and self.tfvarfilepath:
            spcmd.extend(["-var-file", self.tfvarfilepath])

        spcmd.extend(args)

        orig_envs = os.environ.copy()

        envs = {}
        #envs['PATH'] = orig_envs['PATH']
        # set the environment variables from 1Password
        if len(self.op_envs) > 0:
            for key, value in self.op_envs.items():
                if value is not None:  # Ensure value is not None
                    envs[key] = value

        envs['AWS_PROFILE'] = self.aws_profile or ""  # Ensure default empty string
        envs['KUBECONFIG'] = self.kubeconfig or ""    # Ensure default empty string
        envs['KUBE_CONFIG_PATH'] = self.kubeconfig or ""  # Ensure default empty string

        if self.aws_region:
            envs['AWS_REGION'] = self.aws_region

        logging.debug("Executing command in : %s", self.working_dir)
        logging.debug("Executing Terraform with env variables: %s", envs)
        logging.debug("Executing command: %s", spcmd)
        try:
            proc = subprocess.Popen(
                spcmd,
                cwd = self.working_dir,
                stderr = subprocess.PIPE,
                stdin=sys.stdin,
                env=envs
            )
            _, stderr = proc.communicate()

            if self.terraform_debug == "true":
                logging.debug("Terraform stderr: %s", stderr.decode())
            
            if proc.returncode != 0:
                error_message = stderr.decode()
                logging.error("Terraform command failed: %s, Error: [%s]", subcmd, error_message)
                return False
            else:
                return True
        
        except KeyboardInterrupt:
            pass
        finally:
            if caws_region != self.aws_region:
                logging.debug("Unsetting AWS_REGION")
                if 'AWS_REGION' in os.environ:
                    del os.environ['AWS_REGION']
