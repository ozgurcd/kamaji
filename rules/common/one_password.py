import logging
import subprocess

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
