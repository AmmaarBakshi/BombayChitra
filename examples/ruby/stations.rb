# Metro stations and the lines that serve them.
#   ruby stations.rb
require 'json'
require 'net/http'

base = ENV.fetch('BOMBAYCHITRA_URL', 'http://localhost:8080')
data = JSON.parse(Net::HTTP.get(URI("#{base}/api/v1/transit/stations?mode=metro")))

data['items'].sort_by { |s| s['name'].to_s }.each do |station|
  lines = (station['lines'] || []).map { |l| l['label'] }.uniq.join(', ')
  puts format('%-32s %s', station['name'], lines.empty? ? '—' : lines)
end
puts "\n#{data['count']} metro stations"
