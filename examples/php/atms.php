<?php
// Lists ATMs and banks in an area.
//   php atms.php [area-id]      e.g. thane, navi-mumbai, makabo
$base = getenv('BOMBAYCHITRA_URL') ?: 'http://localhost:8080';
$area = $argv[1] ?? 'thane';

$query = http_build_query(['category' => 'atm,bank', 'area' => $area, 'limit' => 25]);
$json = file_get_contents("$base/api/v1/places?$query");
if ($json === false) {
    fwrite(STDERR, "Could not reach $base\n");
    exit(1);
}
$data = json_decode($json, true, flags: JSON_THROW_ON_ERROR);

printf("%d banks and ATMs in %s (first page):\n", $data['count'], $area);
foreach ($data['items'] as $place) {
    printf("  %-5s %-40s %.5f, %.5f\n", strtoupper($place['category']), $place['name'] ?? '(unnamed)', $place['lat'], $place['lon']);
}
