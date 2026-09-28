const fs = require('node:fs');
const path = require('node:path');

function main() {
  const options = Object.create(null);
  const allowed = new Set(['manifest', 'output', 'title', 'columns']);
  for (const argument of process.argv.slice(2)) {
    const separator = argument.indexOf('=');
    const name = argument.slice(2, separator);
    if (!argument.startsWith('--') || separator < 3 || !allowed.has(name) || name in options) {
      throw new Error('expected unique --name=value options');
    }
    options[name] = argument.slice(separator + 1);
  }
  for (const name of allowed) {
    if (!(name in options)) throw new Error(`missing --${name}`);
  }
  const columns = JSON.parse(options.columns);
  if (!Array.isArray(columns) || columns.length === 0 ||
      !columns.every(column => ['path', 'size', 'sha256'].includes(column)) ||
      new Set(columns).size !== columns.length) {
    throw new Error('columns must be a nonempty unique list of path, size, sha256');
  }
  const inventory = JSON.parse(fs.readFileSync(options.manifest, 'utf8'));
  if (inventory.format !== 1 || !Array.isArray(inventory.files)) {
    throw new Error('expected inventory format 1 and a files array');
  }
  const escape = value => String(value).replace(/&/g, '&amp;').replace(/</g, '&lt;')
    .replace(/>/g, '&gt;').replace(/\|/g, '&#124;').replace(/[\r\n]/g, ' ');
  const row = values => `| ${values.map(escape).join(' | ')} |`;
  const lines = [`# ${escape(options.title)}`, '',
    'Inventory only; verification and policy checks run as separate targets.', '',
    row(columns), row(columns.map(() => '---'))];
  for (const file of inventory.files) {
    if (!file || !columns.every(column => Object.hasOwn(file, column))) {
      throw new Error('inventory record is missing a selected column');
    }
    lines.push(row(columns.map(column => file[column])));
  }
  fs.mkdirSync(path.dirname(options.output), {recursive: true});
  fs.writeFileSync(options.output, lines.join('\n') + '\n');
  console.log(`Wrote ${options.output}`);
}

try {
  main();
} catch (error) {
  console.error(`report: ${error.message}`);
  process.exitCode = 2;
}
