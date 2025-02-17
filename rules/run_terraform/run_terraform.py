#!/usr/bin/env python3
"""
Terraform Runner Template
"""

import logging
import argparse
import sys
import os

from terraform import NO_INIT_COMMANDS

logger = logging.getLogger(__name__)

def setupLogging(log_verbosity: str = "INFO") -> None:
    level = getattr(logging, log_verbosity.upper(), None)
    if not isinstance(level, int):
        raise ValueError("Invalid log verbosity: %s" % log_verbosity)
    
    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        formatter = logging.Formatter(
            fmt = '[%(levelname)s] (%(asctime)s): %(message)s',
            datefmt = '%m/%d/%Y %I:%M:%S %p'
        )
        handler.setFormatter(formatter)
        logger.addHandler(handler)  
    
    logger.setLevel(level)

def _main() -> None:
    parser = argparse.ArgumentParser(allow_abbrev=False)

    parser.add_argument('--terraform_executable', required=True)
    parser.add_argument('--log_verbosity')
    parser.add_argument('--aws_profile')
    parser.add_argument('--kubeconfig')
    parser.add_argument('--runtime_vars_file')
    parser.add_argument('--terraform_workspace')
    parser.add_argument('--terraform_backend_config')
    parser.add_argument('--aws_region')
    parser.add_argument('--terraform_debug')

    flags, args = parser.parse_known_args()
    try:
        setupLogging(flags.log_verbosity)
    except ValueError as e:
        print(f"Error setting up logging: {e}", file=sys.stderr)
        sys.exit(1)
    
    logger.debug("log_verbosity: %s" % flags.log_verbosity)

    if flags.aws_region == "":
        logging.error("AWS Region is not set")
        return 1

    ORG_DOMAIN = os.environ.get("KAMAJI_ORGANIZATION_DOMAIN")

    base_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    common_dir = os.path.join(base_dir, "common")
    sys.path.append(common_dir)
    from terraform import TerraformRunner

    if not ORG_DOMAIN:
        logging.error("Can't find KAMAJI_ORGANIZATION_DOMAIN environment variable")
        os.exit(1)
    else:
        logger.debug("ORG_DOMAIN: %s" % ORG_DOMAIN)

    logger.debug("terraform_workspace: %s " % flags.terraform_workspace)
    logger.debug("We got aws profile: %s " % flags.aws_profile)
    logger.debug("We got terraform exec: %s " % flags.terraform_executable)
    logger.debug("We got aws region: %s " % flags.aws_region)
    logger.debug("Terraform debug: %s " % flags.terraform_debug)

    if flags.terraform_backend_config:
        logger.debug("We got terraform backend config: " + flags.terraform_backend_config)

    build_working_dir = os.getcwd()

    terraform_subcmd = ''
    runtimevarsfilepath=''

    if flags.runtime_vars_file:
        runtimevarsfilepath= os.path.join(build_working_dir, flags.runtime_vars_file)   
    
    logger.debug("runtimevarsfilepath: %s", runtimevarsfilepath)
    logger.debug("Verbosity: %s" % flags.log_verbosity)
    logger.debug("Args: %s " % args)

    terraform_executable = [flags.terraform_executable]   
    terraform_executable = [os.path.join(os.getcwd(), flags.terraform_executable)]
    tr = TerraformRunner(
        logger,
        terraform_executable, 
        build_working_dir,
        flags.terraform_backend_config,
        flags.terraform_debug,
        awsProfile=flags.aws_profile,
        awsRegion=flags.aws_region,
        kubeconfig=flags.kubeconfig)

    logger.debug("Terraform Subcommand: [%s]" % terraform_subcmd)
    if len(args) == 0:
        terraform_subcmd = "plan"
    else:
        terraform_subcmd = args[0]
        args.pop(0)

    if terraform_subcmd not in NO_INIT_COMMANDS:
        logger.debug("Command [%s] requires terraform init" % terraform_subcmd)
        tr.init()
    else:
        logger.debug("Command [%s] does not require terraform init" % terraform_subcmd)

    tr.switch_workspace(flags.terraform_workspace)

    if flags.runtime_vars_file:
        tr.setRuntimeVarsFile(runtimevarsfilepath)

    logger.debug("Terraform Subcommand: %s" % terraform_subcmd)
    logger.debug("Terraform args: %s" % args)
    
    exitCode = 0
    if not tr.run(terraform_subcmd, args):
        tr.return_to_default_workspace()
        exitCode = 1
    else:
        tr.return_to_default_workspace()

    sys.exit(exitCode)
## Main
_main()
