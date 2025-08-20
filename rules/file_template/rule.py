import os
from jinja2 import Environment, FileSystemLoader, StrictUndefined
from jinja2.exceptions import UndefinedError

def file_template(**kwargs):
    """
    Renders a Jinja2 template file using provided variables and environment variables.

    Required:
    - template: path to input Jinja2 template file
    - output: path to write rendered file

    Optional:
    - variables: dict of keys to inject (fallbacks to os.environ if not provided)
    - dry_run: if True, prints rendered content instead of writing to file
    """

    template_path = kwargs.get("template")
    output_path = kwargs.get("output")
    user_variables = kwargs.get("variables", {})
    dry_run = kwargs.get("dry_run", False)

    if not template_path or not output_path:
        raise ValueError("Both 'template' and 'output' must be specified.")

    if not os.path.isfile(template_path):
        raise FileNotFoundError(f"Template file '{template_path}' does not exist.")

    # Combine user variables and environment variables
    combined_vars = dict(os.environ)
    combined_vars.update(user_variables)

    # Jinja2 setup
    env = Environment(
        loader=FileSystemLoader(os.path.dirname(template_path)),
        undefined=StrictUndefined,
        autoescape=False,
    )

    template_name = os.path.basename(template_path)

    try:
        template = env.get_template(template_name)
        rendered = template.render(combined_vars)
    except UndefinedError as e:
        raise ValueError(f"Template rendering failed: {e}")

    if dry_run:
        print("Dry Run: Rendered template output:")
        print("────────────────────────────────────")
        print(rendered)
        print("────────────────────────────────────")
    else:
        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        with open(output_path, "w") as f:
            f.write(rendered)
        print(f"Rendered '{template_name}' → '{output_path}'")