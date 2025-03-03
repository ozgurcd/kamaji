import logging
import os
import subprocess
import sys
import json

NO_INIT_COMMANDS=[
    "fmt", 
    "validate", 
    "force-unlock", 
    "taint", 
    "untaint", 
    "show",
    "version"
]

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
    def __init__(self, logger, terraform_path, working_dir, terraform_backend_config, terraform_debug, awsProfile, awsRegion, kubeconfig):        
        self.logger = logger
        self.logger.debug("Initializing TerraformRunner")
        self.terraform_path = terraform_path[0]
        self.working_dir = working_dir
        self.tfvarfilepath = ''
        self.aws_profile = awsProfile
        self.aws_region = awsRegion
        self.kubeconfig = kubeconfig
        self.current_workspace = ""
        self.terraform_backend_config = terraform_backend_config
        self.terraform_debug = terraform_debug
        self.op_envs = [] 

        self.logger.debug("Setting Terraform Path: %s" % self.terraform_path)
        self.logger.debug("Setting Terraform WD: %s" % self.working_dir)
        self.logger.debug("Setting Terraform AWS Profile: %s" % self.aws_profile)
        self.logger.debug("Setting Terraform AWS Region: %s" % self.aws_region)
        self.logger.debug("Setting Terraform terraform_backend_config: %s" % self.terraform_backend_config)

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
        return self.run("init", ["-backend=false"])

    def init(self):
        if self.terraform_backend_config:
            be_config = json.loads(self.terraform_backend_config)
        else:
            be_config = "{}"
        
        if be_config != "{}":
            self.init_with_backend(be_config)
        else:
            self.init_without_backend()

    def setRuntimeVarsFile(self, tfvarfilepath):
        self.logger.debug("Setting Runtime Vars File: %s" % tfvarfilepath)
        self.tfvarfilepath = tfvarfilepath

    def return_to_default_workspace(self):
        self.switch_workspace("default")

    def switch_workspace(self, workspace):
        self.logger.debug("Switching to workspace: %s", workspace)

        if not self.run("workspace", ["select", workspace]):
            self.run("workspace", ["new", workspace])

    def run(self, subcmd, args):
        caws_region = os.environ.get("AWS_REGION", "")
        if self.aws_region == "":
            logging.error("AWS Region is not set")
            return False
        
        self.logger.debug("Current AWS Region: %s", caws_region)
        self.logger.debug("Req Terraform AWS Region: %s", self.aws_region)

        if caws_region != self.aws_region:
            self.logger.debug("Setting AWS Region to: %s", self.aws_region)
            os.environ["AWS_REGION"] = self.aws_region

        self.logger.debug("Executing terraform command: %s", subcmd)
        self.logger.debug("Executing terraform args: %s", args)

        spcmd = [self.terraform_path, subcmd]
        self.logger.debug("spcmd: %s", spcmd)

        if subcmd not in NO_REGION_COMMANDS:
            spcmd.append("-var=aws_region=%s" % self.aws_region)
        self.logger.debug("spcmd: %s", spcmd)

        if subcmd not in NO_INPUT_COMMANDS:
            self.logger.debug("%s command detected, adding -input=false", subcmd)
            spcmd.append("-input=false")

        if subcmd not in NO_ARG_COMMANDS and self.tfvarfilepath:
            spcmd.extend(["-var-file", self.tfvarfilepath])

        spcmd.extend(args)

        envs = {}
        if len(self.op_envs) > 0:
            for key, value in self.op_envs.items():
                if value is not None:  # Ensure value is not None
                    envs[key] = value

        envs['AWS_PROFILE'] = self.aws_profile or "" 
        envs['KUBECONFIG'] = self.kubeconfig or ""   
        envs['KUBE_CONFIG_PATH'] = self.kubeconfig or ""

        if self.aws_region:
            envs['AWS_REGION'] = self.aws_region

        self.logger.debug("Executing command in : %s", self.working_dir)
        self.logger.debug("Executing Terraform with env variables: %s", envs)
        self.logger.debug("Executing command: %s", spcmd)
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
                self.logger.debug("Terraform stderr: %s", stderr.decode())
            
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
                self.logger.debug("Unsetting AWS_REGION")
                if 'AWS_REGION' in os.environ:
                    del os.environ['AWS_REGION']
