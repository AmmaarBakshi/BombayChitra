import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.regex.Pattern;

/**
 * Prints the longest named road bridges. Uses only the JDK (Java 21+); a real application would parse the JSON with
 * Jackson or Gson, or use a client generated from /openapi.json.
 *
 * <pre>java Bridges.java</pre>
 */
public class Bridges {

  public static void main(String[] args) throws Exception {
    String base = System.getenv().getOrDefault("BOMBAYCHITRA_URL", "http://localhost:8080");
    var client = HttpClient.newHttpClient();
    var request = HttpRequest.newBuilder(URI.create(base + "/api/v1/bridges/named?kind=road&limit=10")).build();
    String json = client.send(request, HttpResponse.BodyHandlers.ofString()).body();

    var item = Pattern.compile("\"name\":\"([^\"]+)\",\"kind\":\"[^\"]+\",\"length_m\":([0-9.]+)");
    var matcher = item.matcher(json);
    int rank = 1;
    while (matcher.find()) {
      System.out.printf("%2d. %-45s %6.2f km%n", rank++, matcher.group(1), Double.parseDouble(matcher.group(2)) / 1000);
    }
  }
}
