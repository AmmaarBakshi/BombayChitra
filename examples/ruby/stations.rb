# Metro stations and the lines that serve them.
#   ruby stations.rb
require 'json'
require 'net/http'

base = ENV.fetch('BOMBAYCHITRA_URL', 'http://localhost:8080')
data = JSON.parse(Net::HTTP.get(URI("#{base}/api/v1/transit/stations?mode=metro")))
named = data['items'].select { |s| s['name'] }

named.sort_by { |s| s['name'] }.each do |station|
  lines = (station['lines'] || []).map { |l| l['state'] ? "#{l['label']} (#{l['state']})" : l['label'] }.uniq
  puts format('%-36s %s', station['name'], lines.empty? ? '—' : lines.join(', '))
end
puts "\n#{named.size} named metro stations (#{data['count']} mapped)"
