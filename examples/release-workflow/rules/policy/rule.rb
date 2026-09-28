require 'json'
require 'optparse'

options = {}
parser = OptionParser.new do |opts|
  opts.on('--manifest=PATH') { |value| options[:manifest] = value }
  opts.on('--required_files=JSON') { |value| options[:required] = JSON.parse(value) }
  opts.on('--max_total_bytes=N', Integer) { |value| options[:budget] = value }
  opts.on('--fail_on_empty=BOOL') do |value|
    raise OptionParser::InvalidArgument, 'expected true or false' unless %w[true false].include?(value)
    options[:fail_on_empty] = value == 'true'
  end
end

begin
  parser.parse!
  raise ArgumentError, 'unexpected positional arguments' unless ARGV.empty?
  required = options.fetch(:required)
  unless required.is_a?(Array) && required.all? { |name| name.is_a?(String) }
    raise ArgumentError, 'required_files must be an array of strings'
  end
  data = JSON.parse(File.read(options.fetch(:manifest)))
  raise ArgumentError, 'expected inventory format 1' unless data['format'] == 1
  files = data.fetch('files')
  unless files.is_a?(Array) && files.all? { |item| item.is_a?(Hash) && item['path'].is_a?(String) && item['size'].is_a?(Integer) && item['size'] >= 0 }
    raise ArgumentError, 'invalid file records'
  end
  paths = files.map { |item| item.fetch('path') }
  total = files.inject(0) { |sum, item| sum + item.fetch('size') }
  failures = []
  failures << 'required files missing' unless (required - paths).empty?
  failures << 'duplicate file paths' unless paths.uniq.length == paths.length
  failures << 'empty release or file' if options.fetch(:fail_on_empty) && (files.empty? || files.any? { |item| item['size'].zero? })
  failures << 'total bytes exceed budget' if total > options.fetch(:budget)
  unless failures.empty?
    warn "Policy rejected: #{failures.join('; ')}"
    exit 3
  end
  puts "Policy passed: #{files.length} files, #{total} bytes"
rescue OptionParser::ParseError, JSON::ParserError, KeyError, ArgumentError, SystemCallError => error
  warn "policy: #{error.message}"
  exit 2
end
