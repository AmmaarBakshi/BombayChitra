// Tallest buildings in the Mumbai Metropolitan Region.
// .NET 10 file-based app:  dotnet run Buildings.cs [area]
// JsonDocument reads the response without reflection, so this also works with Native AOT (the default for file-based
// apps).
using System.Text.Json;

var baseUrl = Environment.GetEnvironmentVariable("BOMBAYCHITRA_URL") ?? "http://localhost:8080";
var area = args.Length > 0 ? $"&area={Uri.EscapeDataString(args[0])}" : "";

using var http = new HttpClient { BaseAddress = new Uri($"{baseUrl}/api/v1/") };
await using var body = await http.GetStreamAsync($"buildings?sort=height&named=true&limit=15{area}");
using var doc = await JsonDocument.ParseAsync(body);

var rank = 1;
foreach (var b in doc.RootElement.GetProperty("items").EnumerateArray())
{
    var name = b.TryGetProperty("name", out var n) ? n.GetString() : "Unnamed";
    var height = b.TryGetProperty("height_m", out var h) ? h.GetDouble() : 0;
    var levels = b.TryGetProperty("levels", out var l) ? l.GetInt32().ToString() : "?";
    var locality = b.TryGetProperty("locality", out var loc) ? loc.GetString() : "";
    Console.WriteLine($"{rank++,2}. {name,-32} {height,6:F0} m  {levels,3} floors  {locality}");
}
